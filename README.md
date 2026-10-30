# awdex

[![Docker Build](https://github.com/Awdball/awdex/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/Awdball/awdex/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/awdball/awdex)](https://goreportcard.com/report/github.com/awdball/awdex)

**awdex** is a modern, high-performance, containerized file indexer and media streamer. Inspired by the stylish minimalism and rich file previews of [h5ai](https://github.com/lrsjng/h5ai), and engineered with the scalability and non-blocking concurrency of [copyparty](https://github.com/9001/copyparty) to handle massive directories with hundreds of thousands of files effortlessly.

---

## ✨ Features

- **🎨 Modern, Mobile-First & Desktop-Ready UI**:
  - Clean interface with **Dark / Light** theme support and smooth transitions.
  - **Container-Query Responsive Layout**: Automatically adapts tables into compact two-line mobile rows on narrow screens, slides the folder tree into an off-canvas touch drawer, and uses `100dvh` + safe-area insets for iOS/Android browsers.
  - **Single-Tap Navigation & Touch Gestures**: Open folders and files with a single tap/click, and swipe left/right through image galleries on touchscreens.
- **🌳 Expandable Directory Tree**: Collapsible sidebar tree with lazy-loaded subfolders and item count badges. Navigating deep hierarchies is instantaneous.
- **🚀 Handles Massive Directories**: Non-blocking filesystem directory reads (`<5ms`) and a concurrent background crawler. No PHP timeouts or thread locks.
- **🔍 Indexed & Instant Search**:
  - **In-Folder Quick Filter**: Filter files and folders in the active directory as you type.
  - **Global Library Search (`Ctrl+K` / `⌘K`)**: Sub-millisecond full-text queries across the entire indexed library with category filters (Videos, Audio, Images, Docs, Code, Archives).
- **🎬 Rich Media Previews, Players & Real-Time Transcoding**:
  - **Images**: Lightbox with pan, zoom (mouse wheel / controls), 90° rotation, touch swipe navigation, and arrow key support.
  - **Videos (Native + On-the-Fly FFmpeg Transcoding)**:
    - Direct HTTP byte-range streaming (`206 Partial Content`) for browser-native formats (`MP4`, `WebM`, `MOV`, `MKV`).
    - **Real-Time FFmpeg Transcoding (`/api/transcode`)**: Automatically transcodes legacy and non-browser codecs (`WMV`, `AVI`, `ASF`, `FLV`, `MPEG`, `MPG`, `TS`, `M2TS`, `VOB`, `DIVX`, `3GP`) on the fly into zero-latency fragmented MP4 (`H.264 + AAC`) so every video plays directly in the browser without downloading.
    - **Animated WebP Scrub Previews**: Hovering over video cards in Grid View plays a multi-segment animated WebP preview.
  - **Audio**: Docked bottom music player with continuous background playback while browsing directories, auto-advancing playlist queue, ID3 album art extraction, seekbar, volume, loop, and shuffle.
  - **PDFs**: Embedded native PDF viewer.
  - **Code & Text**: Syntax highlighting for 100+ languages (`highlight.js`), word wrap toggle, and one-click copy-to-clipboard.
  - **Markdown**: Formatted GFM HTML view with rendered/raw source toggle.
  - **ZIP Archives**: Inspect internal archive file trees and sizes without extracting or downloading the archive.
- **🔗 Public Share Links & Pluggable Outbound Tunnels**:
  - Create expiring, download-limited, or password-protected public share links (`/s/<token>`) for any file or folder (folders stream as on-the-fly `.zip` downloads; legacy videos stream via live FFmpeg transcoding).
  - **Isolated Share-Only Port (`:8081`)**: Dedicated HTTP listener that *only* serves `/s/<token>` routes—never exposing `/api/*`, `/raw/*`, or the directory browser UI to public tunnels.
  - **Pluggable Zero-Install Outbound Tunnels**: Built-in support for **Cloudflare Quick Tunnels (`cloudflared`)** (auto-downloaded to `/data/bin/cloudflared`, auto-started on first share, and auto-stopped when all shares expire/revoke), **OpenZiti (`zrok`)**, and **Custom Domain / Tailscale Funnel**.
- **📤 Streaming Drag-and-Drop Uploads**: Drop files anywhere onto the browser window or use the Upload modal with real-time progress. Can be disabled globally via `-read-only` / `AWDEX_READ_ONLY=true`.
- **💾 Strict Content & Asset Separation**:
  - Your content directory (`/content`) can be mounted **read-only (`:ro`)**.
  - All generated assets (thumbnails, animated video scrubs, and SQLite database) are stored in a separate data/cache volume (`/data`).
- **🗄️ Database Flexibility**:
  - **SQLite (Default)**: Embedded pure-Go SQLite (`modernc.org/sqlite`), zero configuration required.
  - **PostgreSQL**: Optional external PostgreSQL connection (`jackc/pgx/v5`) for Kubernetes clusters, multi-node deployments, or NFS environments where SQLite file locking is undesirable.
- **📦 Single Static Binary**: The entire Vue 3 + Vite Single Page Application is embedded directly inside the Go binary via `embed.FS`.

---

## 🚀 Quickstart

### 1. Docker CLI

Point `awdex` at any directory on your host:

```bash
docker run -d \
  --name awdex \
  -p 8080:8080 \
  -v /path/to/my/files:/content:ro \
  -v /path/to/awdex-data:/data \
  ghcr.io/awdball/awdex:latest
```

Open **`http://localhost:8080`** in your browser.

---

### 2. Docker Compose

```yaml
services:
  awdex:
    image: ghcr.io/awdball/awdex:latest
    container_name: awdex
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - AWDEX_TITLE=awdex
      - AWDEX_ROOT_DIR=/content
      - AWDEX_DATA_DIR=/data
      - AWDEX_READ_ONLY=false
      - AWDEX_ENABLE_THUMBNAILS=true
      - AWDEX_ENABLE_VIDEO_SCRUB=true
    volumes:
      # Content directory (mount :ro if read-only)
      - /mnt/storage:/content:ro
      # Separate persistent directory for database, thumbnails, and scrub previews
      - /var/lib/awdex:/data
```

```bash
docker compose up -d
```

---

### 3. Kubernetes Deployment (with External PostgreSQL)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: awdex
spec:
  replicas: 1
  selector:
    matchLabels:
      app: awdex
  template:
    metadata:
      labels:
        app: awdex
    spec:
      containers:
        - name: awdex
          image: ghcr.io/awdball/awdex:latest
          ports:
            - containerPort: 8080
          env:
            - name: AWDEX_ROOT_DIR
              value: "/content"
            - name: AWDEX_DATA_DIR
              value: "/data"
            - name: AWDEX_DATABASE_URL
              value: "postgres://awdex:secret@postgres-rw:5432/awdex?sslmode=disable"
          volumeMounts:
            - name: content
              mountPath: /content
            - name: data
              mountPath: /data
      volumes:
        - name: content
          persistentVolumeClaim:
            claimName: media-pvc
        - name: data
          persistentVolumeClaim:
            claimName: awdex-data-pvc
```

---

### 4. Build from Source

Requires **Go 1.23+**, **Node.js 20+**, **FFmpeg** (for video thumbnails, animated scrubs, and real-time transcoding), and **Poppler (`pdftoppm`)** (optional, for PDF first-page thumbnails).

```bash
git clone https://github.com/Awdball/awdex.git
cd awdex

# 1. Build Vue 3 SPA
cd web
npm install
npm run build
cd ..

# 2. Build static Go binary with embedded SPA
CGO_ENABLED=0 go build -ldflags="-s -w" -o awdex ./cmd/awdex

# 3. Run
./awdex -root /path/to/files -data /path/to/cache -port 8080
```

---

## ⚙️ Configuration Reference

All settings can be configured via environment variables or CLI flags (CLI flags take precedence):

| CLI Flag | Environment Variable | Default | Description |
| :--- | :--- | :--- | :--- |
| `-root` | `AWDEX_ROOT_DIR` | `/content` | Directory containing the files to index and serve. |
| `-data` | `AWDEX_DATA_DIR` | `/data` | Directory where thumbnails, video scrubs, and SQLite database are stored. |
| `-port` | `AWDEX_PORT` | `8080` | Port for the HTTP server to listen on. |
| `-host` | `AWDEX_HOST` | `0.0.0.0` | Host interface to bind. |
| `-title` | `AWDEX_TITLE` | `awdex` | Site title displayed in the navbar and browser tab. |
| `-read-only` | `AWDEX_READ_ONLY` | `false` | When `true`, disables uploads, folder creation, and deletion. |
| `-db` | `AWDEX_DATABASE_URL` | *(empty)* | Optional PostgreSQL DSN (`postgres://user:pass@host:5432/dbname?sslmode=disable`). Uses embedded SQLite at `/data/awdex.db` when omitted. |
| | `AWDEX_ENABLE_THUMBNAILS` | `true` | Generate image, audio (ID3 cover), video, and PDF thumbnails. |
| | `AWDEX_ENABLE_VIDEO_SCRUB` | `true` | Generate animated WebP scrub previews on video card hover. |
| | `AWDEX_INDEX_INTERVAL` | `10m` | Interval between background filesystem sync crawls. |

---

## 🔌 REST API Endpoints

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/info` | Server metadata, read-only status, database driver, and live index statistics. |
| `GET` | `/api/browse?path=<dir>` | Lists immediate directory contents, breadcrumbs, and folder item counts. |
| `GET` | `/api/tree?path=<dir>` | Returns subdirectories for the expandable left-hand folder tree. |
| `GET` | `/api/search?q=<query>&category=<cat>` | Searches indexed files and directories with optional category filter. |
| `GET` | `/api/thumb?path=<file>` | Generates (and caches in `/data/thumbnails`) a `360px` JPEG thumbnail. |
| `GET` | `/api/scrub?path=<video>` | Generates (and caches in `/data/thumbnails`) an animated WebP scrub preview. |
| `GET` | `/api/transcode?path=<video>` | Streams real-time FFmpeg fragmented MP4 (`H.264 + AAC`) transcode for non-native video formats. |
| `GET` | `/api/archive?path=<zip>` | Returns the internal file table of contents for `.zip` archives. |
| `GET` | `/raw/<path>` | Serves raw file bytes with full `Range` / `206 Partial Content` support (`?download=1` forces attachment). |
| `POST` | `/api/upload?path=<dir>` | Multipart streaming file upload into target directory (disabled in read-only mode). |
| `POST` | `/api/mkdir` | Creates a new subdirectory (disabled in read-only mode). |
| `DELETE` | `/api/delete?path=<item>` | Deletes a file or directory and removes its index entry (disabled in read-only mode). |
| `GET/POST/DELETE` | `/api/shares` | List active public share links, create a new share link (`path`, `expires_in_hours`, `max_downloads`, `password`), or revoke a share (`?id=<token>`). |
| `POST` | `/api/shares/config` | Configure outbound tunnel provider (`cloudflared`, `zrok`, `custom`, `none`) and control tunnel lifecycle (`start` / `stop`). |
| `GET/POST` | `/s/<token>` | Isolated public share landing page, password verification, raw/transcoded media preview, and single-file or folder `.zip` download (served on both `:8080` and isolated `:8081`). |

---

## 🏗️ Architecture

```
┌───────────────────────────────────────────────────────────────────┐
│                          awdex Container                          │
│                                                                   │
│   ┌───────────────────────────────────────────────────────────┐   │
│   │                   Vue 3 Embedded SPA                      │   │
│   │  (Drawer Tree · Container-Query Table/Grid · Media Modal) │   │
│   └─────────────────────────────┬─────────────────────────────┘   │
│                                 │ JSON / Range / MP4 Streams      │
│   ┌─────────────────────────────▼─────────────────────────────┐   │
│   │                      Go HTTP Server                       │   │
│   │  - Direct os.ReadDir (<5ms directory browsing)            │   │
│   │  - http.ServeContent (Byte-Range streaming for 4K video)  │   │
│   │  - /api/transcode (On-the-fly FFmpeg H.264+AAC MP4 pipe)  │   │
│   │  - Streaming multipart uploads                            │   │
│   └───────────────┬───────────────────────────┬───────────────┘   │
│                   │                           │                   │
│   ┌───────────────▼─────────────┐   ┌─────────▼───────────────┐   │
│   │     Background Indexer      │   │    Thumbnail Engine     │   │
│   │  (Concurrent FS Crawler)    │   │ (Image / ID3 / FFmpeg)  │   │
│   └───────────────┬─────────────┘   └─────────┬───────────────┘   │
└───────────────────┼───────────────────────────┼───────────────────┘
                    │                           │
          ┌─────────▼──────────┐      ┌─────────▼─────────┐
          │ SQLite / Postgres  │      │  Thumbnails Dir   │
          │ (/data or external)│      │ (/data/thumbnails)│
          └────────────────────┘      └───────────────────┘
                    ▲
                    │ (Separate Asset Directory)
          ┌─────────┴──────────┐
          │   /content (:ro)   │
          │ (Indexed Directory)│
          └────────────────────┘
```

---

## 🛡️ Security

- **Path Traversal Protection**: All relative paths are cleaned and verified against the canonical root directory prefix before any filesystem operation.
- **Read-Only Enforcement**: When running with `-read-only` or `AWDEX_READ_ONLY=true`, write endpoints (`POST /api/upload`, `POST /api/mkdir`, `DELETE /api/delete`) return `403 Forbidden` and upload UI controls are hidden.
- **Isolated Asset Storage**: The indexed content directory is never polluted with hidden `.awdex` folders or cache databases when `/data` is pointed at a separate path.
- **Embedded Static Assets**: Zero dynamic server-side template execution.

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
