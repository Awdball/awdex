# ==========================================
# Stage 1: Build Frontend Assets
# ==========================================
FROM node:22-alpine AS frontend-builder

WORKDIR /app/web

COPY web/package.json ./
RUN npm install --no-audit --no-fund

COPY web/ ./
RUN npm run build

# ==========================================
# Stage 2: Compile Static Go Binary
# ==========================================
FROM alpine:edge AS backend-builder

WORKDIR /app

RUN apk add --no-cache go git ca-certificates tzdata

COPY go.mod go.sum ./
ENV CGO_ENABLED=0 GOTOOLCHAIN=auto
RUN go mod download

# Copy source code and frontend build output
COPY cmd/ cmd/
COPY internal/ internal/
COPY web/embed.go web/
COPY --from=frontend-builder /app/web/dist web/dist

# Build fully static binary
RUN go build -ldflags="-s -w -X main.version=1.0.0" -o /app/bin/awdex ./cmd/awdex

# ==========================================
# Stage 3: Minimal Production Image
# ==========================================
FROM alpine:3.21

# Install runtime utilities for media preview & transcoding
RUN apk add --no-cache \
    ffmpeg \
    poppler-utils \
    ca-certificates \
    tzdata

WORKDIR /app

COPY --from=backend-builder /app/bin/awdex /app/awdex

ENV AWDEX_CONTENT_DIR=/content \
    AWDEX_DATA_DIR=/data \
    AWDEX_PORT=8080

# Volume mountpoints
VOLUME ["/content", "/data"]

EXPOSE 8080

ENTRYPOINT ["/app/awdex"]
