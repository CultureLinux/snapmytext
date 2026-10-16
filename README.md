# SnapMyText

Application image-vers-texte dans un conteneur unique : interface web embarquée, API HTTP en Go et OCR Tesseract avec les modèles `tessdata_best` français et anglais.

## Start

```bash
docker compose up -d --build
```

Ouvrez http://127.0.0.1:8080 depuis la machine Docker. Pour un accès distant, placez Nginx ou Traefik devant le service. Par défaut, Compose n’écoute que sur l’interface locale.

Une image préconstruite peut également être lancée avec :

```bash
docker run --rm -p 127.0.0.1:8080:8080 --read-only --tmpfs /tmp:size=32m clinuxfr/snapmytext:latest
```

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
- Build downloads the `main` revision of `tessdata_best`; pin model URLs to a reviewed commit and verify checksums for reproducible production builds.

## Development

```bash
go run .
```

Requires locally installed Tesseract and `fra`/`eng` language data. Go 1.23 or newer.

## Publication Docker Hub

Le workflow `.github/workflows/docker-publish.yml` vérifie le code, puis publie une image multi-architecture sur `clinuxfr/snapmytext` lors d’un push sur `main`, d’un tag `v*.*.*` ou d’un lancement manuel.

Ajoutez ces secrets dans **Settings → Secrets and variables → Actions** :

- `DOCKERHUB_USERNAME` : votre identifiant Docker Hub ;
- `DOCKERHUB_TOKEN` : un jeton d’accès Docker Hub disposant du droit d’écriture.

La branche principale publie `latest` et `sha-…`. Un tag comme `v1.2.3` publie également `1.2.3` et `1.2`.
