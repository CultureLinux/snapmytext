# syntax=docker/dockerfile:1

# ==================================================
# BUILD : Go 1.26
# ==================================================
FROM golang:1.26-bookworm AS build

WORKDIR /src

# Dépendances Go
COPY go.mod go.sum ./
RUN go mod download

# Sources
COPY main.go ./
COPY web/ ./web/

# Compilation
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /light-ocr .

# ==================================================
# RUNTIME : Debian + Tesseract Best
# ==================================================
FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive \
    TESSDATA_PREFIX=/usr/share/tesseract-ocr/5/tessdata \
    OMP_THREAD_LIMIT=1 \
    OCR_MAX_CONCURRENT=1

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      tesseract-ocr \
      ca-certificates \
      curl \
    && mkdir -p "$TESSDATA_PREFIX" \
    && curl -fLsS --retry 3 \
      https://raw.githubusercontent.com/tesseract-ocr/tessdata_best/main/fra.traineddata \
      -o "$TESSDATA_PREFIX/fra.traineddata" \
    && curl -fLsS --retry 3 \
      https://raw.githubusercontent.com/tesseract-ocr/tessdata_best/main/eng.traineddata \
      -o "$TESSDATA_PREFIX/eng.traineddata" \
    && apt-get purge -y --auto-remove curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /light-ocr /usr/local/bin/light-ocr

USER 65534:65534

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/light-ocr"]