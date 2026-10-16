# Light OCR

Single-container image-to-text app: embedded web UI, Go HTTP API and Tesseract OCR with `tessdata_fast` French and English.

## Start

```bash
docker compose up -d --build
```

Visit http://127.0.0.1:8080 (from the Docker host). For remote access, put Nginx or Traefik in front of the service. The default Compose binding is loopback only.

## API

```bash
curl -F 'image=@capture.png' -F 'lang=fra+eng' http://127.0.0.1:8080/api/ocr
```

Returns JSON `{ "text": "...", "duration_ms": 123 }`. Languages: `fra`, `eng`, `fra+eng`.

## Limits

- PNG and JPEG only, 10 MiB HTTP request maximum, 12 megapixels decoded dimensions.
- One OCR request at a time by default; concurrent requests receive HTTP 429.
- 20-second OCR timeout; uploads held in temporary files deleted after request.
- Read-only container root; `/tmp` tmpfs (32 MiB); 1 CPU / 512 MiB RAM caps.
- No authentication: **add reverse-proxy rate limiting, TLS, and preferably access control before public exposure**.
- The OCR subprocess can temporarily allocate significant memory; 512 MiB is a starting point, not a guarantee for all inputs.
- Build downloads the `main` revision of `tessdata_fast`; pin model URLs to a reviewed commit and verify checksums for reproducible production builds.

## Development

```bash
go run .
```

Requires locally installed Tesseract and `fra`/`eng` language data. Go 1.23 or newer.
