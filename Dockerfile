# ==========================================
# Stage 1: Build Frontend Assets
# ==========================================
FROM node:22-alpine AS frontend-builder

WORKDIR /app/web

COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# ==========================================
# Stage 2: Compile Static Go Binary
# ==========================================
FROM golang:1.24-alpine AS backend-builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

# Copy source code and frontend build output
COPY cmd/ cmd/
COPY internal/ internal/
COPY web/embed.go web/
COPY --from=frontend-builder /app/web/dist web/dist

# Build fully static binary
ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w -X main.version=1.0.0" -o /app/bin/awdex ./cmd/awdex

# ==========================================
# Stage 3: Minimal Production Image
# ==========================================
FROM alpine:3.21

# Install runtime utilities for media preview generation
RUN apk add --no-cache \
    ffmpeg \
    poppler-utils \
    ca-certificates \
    tzdata

WORKDIR /app

COPY --from=backend-builder /app/bin/awdex /app/awdex

ENV AWDEX_ROOT_DIR=/content \
    AWDEX_DATA_DIR=/data \
    AWDEX_PORT=8080 \
    AWDEX_HOST=0.0.0.0

# Volume mountpoints
VOLUME ["/content", "/data"]

EXPOSE 8080

ENTRYPOINT ["/app/awdex"]
