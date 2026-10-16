package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/draw"
)

//go:embed web/*
var assets embed.FS

const maxUpload = 10 << 20 // 10 MiB HTTP body
const maxPixels = 12_000_000

type response struct {
	Text       string `json:"text,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
}

func send(w http.ResponseWriter, status int, r response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(r)
}
func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
func upscaleForOCR(inputPath string, width, height int) (string, error) {
	// Heuristique initiale pour les petites captures.
	if width >= 1200 {
		return inputPath, nil
	}

	// Évite de dépasser 12 MP après agrandissement.
	if int64(width)*int64(height)*4 > 12_000_000 {
		return inputPath, nil
	}

	f, err := os.Open(inputPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	src, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}

	dst := image.NewRGBA(image.Rect(0, 0, width*2, height*2))

	// Interpolation Catmull-Rom pour le texte.
	draw.CatmullRom.Scale(
		dst, dst.Bounds(),
		src, src.Bounds(),
		draw.Over, nil,
	)

	out, err := os.CreateTemp("", "ocr-upscaled-*.png")
	if err != nil {
		return "", err
	}

	path := out.Name()

	if err := png.Encode(out, dst); err != nil {
		out.Close()
		os.Remove(path)
		return "", err
	}

	if err := out.Close(); err != nil {
		os.Remove(path)
		return "", err
	}

	return path, nil
}
func main() {
	limit, err := strconv.Atoi(env("OCR_MAX_CONCURRENT", "1"))
	if err != nil || limit < 1 || limit > 8 {
		log.Fatal("OCR_MAX_CONCURRENT must be 1..8")
	}
	queue := make(chan struct{}, limit)
	mux := http.NewServeMux()
	webAssets, err := fs.Sub(assets, "web")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("GET /", http.FileServer(http.FS(webAssets)))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("POST /api/ocr", func(w http.ResponseWriter, r *http.Request) {
		select {
		case queue <- struct{}{}:
			defer func() { <-queue }()
		default:
			send(w, 429, response{Error: "Serveur occupé, réessayez."})
			return
		}
		start := time.Now()
		r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			send(w, 413, response{Error: "Image invalide ou trop volumineuse (10 Mo max)."})
			return
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		file, header, err := r.FormFile("image")
		if err != nil {
			send(w, 400, response{Error: "Champ image manquant."})
			return
		}
		defer file.Close()
		if header.Size > maxUpload {
			send(w, 413, response{Error: "Image trop volumineuse."})
			return
		}
		cfg, format, err := image.DecodeConfig(file)
		if err != nil || (format != "jpeg" && format != "png") {
			send(w, 415, response{Error: "PNG ou JPEG uniquement."})
			return
		}
		if cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > maxPixels {
			send(w, 413, response{Error: "Image limitée à 12 mégapixels."})
			return
		}
		if _, err = file.Seek(0, io.SeekStart); err != nil {
			send(w, 500, response{Error: "Lecture impossible."})
			return
		}
		suffix := ".png"
		if format == "jpeg" {
			suffix = ".jpg"
		}
		tmp, err := os.CreateTemp("", "ocr-*"+suffix)
		if err != nil {
			send(w, 500, response{Error: "Espace temporaire indisponible."})
			return
		}
		name := tmp.Name()
		defer os.Remove(name)
		_, copyErr := io.Copy(tmp, io.LimitReader(file, maxUpload+1))
		closeErr := tmp.Close()
		if copyErr != nil || closeErr != nil {
			send(w, 500, response{Error: "Impossible de préparer l'image."})
			return
		}
		lang := r.FormValue("lang")
		if lang != "fra" && lang != "eng" && lang != "fra+eng" {
			lang = "fra+eng"
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		ocrPath, err := upscaleForOCR(name, cfg.Width, cfg.Height)
		if err != nil {
			send(w, 500, response{Error: "Image preprocessing failed."})
			return
		}
		if ocrPath != name {
			log.Printf(
				"OCR upscale: %dx%d -> %dx%d",
				cfg.Width,
				cfg.Height,
				cfg.Width*2,
				cfg.Height*2,
			)
			defer os.Remove(ocrPath)
		} else {
			log.Printf(
				"OCR original: %dx%d (no upscale)",
				cfg.Width,
				cfg.Height,
			)
		}
		cmd := exec.CommandContext(
			ctx,
			"tesseract",
			filepath.Clean(ocrPath),
			"stdout",
			"-l", lang,
			"--oem", "1",
			"--psm", "6",
			"-c", "preserve_interword_spaces=1",
		)
		cmd.Env = append(os.Environ(), "OMP_THREAD_LIMIT=1")
		output, err := cmd.Output()
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				send(w, 504, response{Error: "Délai OCR dépassé (20 s)."})
				return
			}
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				log.Printf("tesseract failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
			} else {
				log.Printf("tesseract: %v", err)
			}
			send(w, 500, response{Error: "Échec de la reconnaissance OCR."})
			return
		}
		send(w, 200, response{Text: strings.TrimSpace(string(output)), DurationMS: time.Since(start).Milliseconds()})
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' blob: data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		mux.ServeHTTP(w, r)
	})
	server := &http.Server{Addr: ":8080", Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 60 * time.Second}
	fmt.Println("SnapMyText listening on :8080")
	log.Fatal(server.ListenAndServe())
}
