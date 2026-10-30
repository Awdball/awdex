package server

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/awdball/awdex/internal/share"
	"github.com/awdball/awdex/internal/util"
	"github.com/dustin/go-humanize"
)

type createShareRequest struct {
	Path           string  `json:"path"`
	ExpiresInHours float64 `json:"expires_in_hours"`
	MaxDownloads   int     `json:"max_downloads"`
	Password       string  `json:"password"`
}

type updateShareConfigRequest struct {
	Provider      string `json:"provider"`
	PublicBaseURL string `json:"public_base_url"`
	ZrokToken     string `json:"zrok_token"`
	Action        string `json:"action"` // "", "start", "stop"
}

func (s *Server) handleSharesAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		shares := s.shares.ListShares()
		st, settings := s.shares.Status()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"shares":   shares,
			"tunnel":   st,
			"settings": settings,
		})

	case http.MethodPost:
		var req createShareRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}
		relPath := util.CleanRelativePath(req.Path)
		fullPath := filepath.Join(s.cfg.RootDir, filepath.FromSlash(strings.TrimPrefix(relPath, "/")))

		fi, err := os.Stat(fullPath)
		if err != nil {
			http.Error(w, "Target file or folder not found", http.StatusNotFound)
			return
		}

		cat, _, _ := util.DetectCategory(fi.Name(), fi.IsDir())
		link, tunErr := s.shares.CreateShare(
			relPath,
			fi.Name(),
			fi.IsDir(),
			fi.Size(),
			cat,
			req.ExpiresInHours,
			req.MaxDownloads,
			req.Password,
		)

		st, settings := s.shares.Status()
		resp := map[string]any{
			"share":    link,
			"tunnel":   st,
			"settings": settings,
		}
		if tunErr != nil {
			resp["warning"] = tunErr.Error()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)

	case http.MethodDelete:
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			http.Error(w, "Missing share id", http.StatusBadRequest)
			return
		}
		s.shares.DeleteShare(id)
		st, settings := s.shares.Status()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deleted":  id,
			"shares":   s.shares.ListShares(),
			"tunnel":   st,
			"settings": settings,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSharesConfigAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req updateShareConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.Provider != "" || req.PublicBaseURL != "" || req.ZrokToken != "" {
		s.shares.UpdateSettings(req.Provider, req.PublicBaseURL, req.ZrokToken)
	}

	var actionErr error
	if req.Action == "stop" {
		s.shares.StopTunnel()
	} else if req.Action == "start" {
		_, actionErr = s.shares.EnsureTunnel()
	}

	st, settings := s.shares.Status()
	resp := map[string]any{
		"shares":   s.shares.ListShares(),
		"tunnel":   st,
		"settings": settings,
	}
	if actionErr != nil {
		resp["warning"] = actionErr.Error()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func needsTranscodeExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".wmv", ".avi", ".flv", ".mpg", ".mpeg", ".m2ts", ".ts", ".vob", ".ogv", ".3gp", ".divx":
		return true
	}
	return false
}

func (s *Server) checkShareAuth(w http.ResponseWriter, r *http.Request, link *share.ShareLink) bool {
	if link.PasswordHash == "" {
		return true
	}
	cookieName := "awdex_share_" + link.ID
	if c, err := r.Cookie(cookieName); err == nil && c.Value == link.PasswordHash {
		return true
	}
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		pw := r.FormValue("password")
		if share.HashPassword(pw) == link.PasswordHash {
			http.SetCookie(w, &http.Cookie{
				Name:     cookieName,
				Value:    link.PasswordHash,
				Path:     "/s/" + link.ID,
				HttpOnly: true,
				MaxAge:   86400 * 7,
			})
			return true
		}
	}
	return false
}

func (s *Server) handlePublicShare(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/s/")
	parts := strings.SplitN(trimmed, "/", 2)
	token := strings.TrimSpace(parts[0])
	subAction := ""
	if len(parts) > 1 {
		subAction = parts[1]
	}

	if token == "" {
		http.NotFound(w, r)
		return
	}

	link, ok := s.shares.GetShare(token)
	if !ok {
		s.renderShareErrorPage(w, http.StatusNotFound, "Share Link Expired or Not Found", "This shared link has expired, reached its download limit, or was revoked by the owner.")
		return
	}

	if !s.checkShareAuth(w, r, link) {
		s.renderSharePasswordPage(w, link, r.Method == http.MethodPost)
		return
	}

	fullPath := filepath.Join(s.cfg.RootDir, filepath.FromSlash(strings.TrimPrefix(link.Path, "/")))
	fi, err := os.Stat(fullPath)
	if err != nil {
		s.renderShareErrorPage(w, http.StatusNotFound, "File Not Found", "The underlying file or folder no longer exists on the server.")
		return
	}

	switch {
	case subAction == "download" || subAction == "raw":
		if fi.IsDir() {
			s.serveSharedFolderZip(w, r, link, fullPath)
			return
		}
		if subAction == "download" {
			s.shares.IncrementDownload(link.ID)
		}
		f, err := os.Open(fullPath)
		if err != nil {
			http.Error(w, "Cannot open file", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		_, mimeType, _ := util.DetectCategory(fi.Name(), false)
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Accept-Ranges", "bytes")
		if subAction == "download" {
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fi.Name()))
		} else {
			w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, fi.Name()))
		}
		http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)

	case subAction == "transcode":
		if fi.IsDir() || !s.thumbs.HasFFmpeg() {
			http.Error(w, "Cannot transcode", http.StatusBadRequest)
			return
		}
		args := []string{
			"-hide_banner", "-loglevel", "error",
			"-i", fullPath,
			"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency", "-crf", "23", "-pix_fmt", "yuv420p",
			"-c:a", "aac", "-b:a", "128k", "-ac", "2",
			"-movflags", "frag_keyframe+empty_moov+default_base_moof",
			"-f", "mp4", "pipe:1",
		}
		cmd := exec.CommandContext(r.Context(), "ffmpeg", args...)
		cmd.Stdout = w
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Cache-Control", "no-cache")
		_ = cmd.Run()

	case strings.HasPrefix(subAction, "file/"):
		if !fi.IsDir() {
			http.NotFound(w, r)
			return
		}
		relChild := util.CleanRelativePath(strings.TrimPrefix(subAction, "file/"))
		childFull := filepath.Join(fullPath, filepath.FromSlash(strings.TrimPrefix(relChild, "/")))
		// Verify childFull is inside fullPath
		if !strings.HasPrefix(filepath.Clean(childFull), filepath.Clean(fullPath)) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		cfi, err := os.Stat(childFull)
		if err != nil || cfi.IsDir() {
			http.NotFound(w, r)
			return
		}
		s.shares.IncrementDownload(link.ID)
		cf, err := os.Open(childFull)
		if err != nil {
			http.Error(w, "Cannot open file", http.StatusInternalServerError)
			return
		}
		defer cf.Close()
		_, mimeType, _ := util.DetectCategory(cfi.Name(), false)
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, cfi.Name()))
		http.ServeContent(w, r, cfi.Name(), cfi.ModTime(), cf)

	default:
		s.renderShareLandingPage(w, link, fullPath, fi)
	}
}

func (s *Server) serveSharedFolderZip(w http.ResponseWriter, r *http.Request, link *share.ShareLink, folderPath string) {
	s.shares.IncrementDownload(link.ID)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, link.Name))

	zw := zip.NewWriter(w)
	defer zw.Close()

	_ = filepath.Walk(folderPath, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}
		rel, err := filepath.Rel(folderPath, p)
		if err != nil {
			return nil
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return nil
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		_, _ = io.Copy(writer, f)
		f.Close()
		return nil
	})
}

func (s *Server) renderShareErrorPage(w http.ResponseWriter, code int, title, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s — awdex</title>
<style>
  body { margin:0; min-height:100dvh; display:flex; align-items:center; justify-content:center; background:#090d16; color:#f8fafc; font-family:system-ui,-apple-system,sans-serif; padding:20px; box-sizing:border-box; }
  .card { background:#1e293b; border:1px solid #334155; border-radius:12px; padding:32px; max-width:440px; width:100%%; text-align:center; box-shadow:0 20px 25px -5px rgba(0,0,0,0.5); }
  h1 { font-size:18px; margin:0 0 10px; }
  p { color:#94a3b8; font-size:14px; line-height:1.5; margin:0; }
</style>
</head>
<body><div class="card"><h1>%s</h1><p>%s</p></div></body></html>`, html.EscapeString(title), html.EscapeString(title), html.EscapeString(msg))
}

func (s *Server) renderSharePasswordPage(w http.ResponseWriter, link *share.ShareLink, failed bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	errBanner := ""
	if failed {
		errBanner = `<div style="background:rgba(239,68,68,0.15);color:#f87171;padding:8px 12px;border-radius:6px;font-size:13px;margin-bottom:14px;">Incorrect password. Please try again.</div>`
	}
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Password Required — %s</title>
<style>
  body { margin:0; min-height:100dvh; display:flex; align-items:center; justify-content:center; background:#090d16; color:#f8fafc; font-family:system-ui,-apple-system,sans-serif; padding:20px; box-sizing:border-box; }
  .card { background:#1e293b; border:1px solid #334155; border-radius:12px; padding:28px; max-width:400px; width:100%%; box-shadow:0 20px 25px -5px rgba(0,0,0,0.5); }
  h2 { font-size:17px; margin:0 0 6px; }
  p { color:#94a3b8; font-size:13px; margin:0 0 18px; word-break:break-all; }
  input { width:100%%; box-sizing:border-box; padding:10px 12px; border-radius:8px; border:1px solid #334155; background:#0f172a; color:#f8fafc; font-size:14px; margin-bottom:12px; }
  button { width:100%%; padding:10px 16px; border:none; border-radius:8px; background:#3b82f6; color:#fff; font-weight:600; font-size:14px; cursor:pointer; }
  button:hover { background:#2563eb; }
</style>
</head>
<body>
  <form class="card" method="POST" action="/s/%s">
    <h2>Protected Share</h2>
    <p>Enter the password to access <strong>%s</strong></p>
    %s
    <input type="password" name="password" placeholder="Password" autofocus required />
    <button type="submit">Unlock Share</button>
  </form>
</body></html>`, html.EscapeString(link.Name), html.EscapeString(link.ID), html.EscapeString(link.Name), errBanner)
}

func (s *Server) renderShareLandingPage(w http.ResponseWriter, link *share.ShareLink, fullPath string, fi os.FileInfo) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	escapedName := html.EscapeString(link.Name)
	sizeStr := humanize.Bytes(uint64(fi.Size()))

	var previewHTML string
	if fi.IsDir() {
		entries, _ := os.ReadDir(fullPath)
		var rows strings.Builder
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") || e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			en := html.EscapeString(e.Name())
			rows.WriteString(fmt.Sprintf(`<div class="folder-item">
				<span class="fname">%s</span>
				<span class="fsize">%s</span>
				<a class="fbtn" href="/s/%s/file/%s">Download</a>
			</div>`, en, humanize.Bytes(uint64(info.Size())), link.ID, en))
		}
		if rows.Len() == 0 {
			rows.WriteString(`<div style="padding:24px;text-align:center;color:#94a3b8;">No direct files in folder</div>`)
		}
		previewHTML = fmt.Sprintf(`<div class="folder-list">%s</div>`, rows.String())
	} else {
		switch link.Category {
		case "video":
			src := fmt.Sprintf("/s/%s/raw", link.ID)
			if needsTranscodeExt(link.Name) && s.thumbs.HasFFmpeg() {
				src = fmt.Sprintf("/s/%s/transcode", link.ID)
			}
			previewHTML = fmt.Sprintf(`<video class="media-player" controls playsinline preload="metadata" src="%s"></video>`, src)
		case "image":
			previewHTML = fmt.Sprintf(`<img class="media-img" src="/s/%s/raw" alt="%s" />`, link.ID, escapedName)
		case "audio":
			previewHTML = fmt.Sprintf(`<div style="padding:32px;width:100%%;max-width:520px;"><audio style="width:100%%;" controls src="/s/%s/raw"></audio></div>`, link.ID)
		case "pdf":
			previewHTML = fmt.Sprintf(`<iframe class="pdf-frame" src="/s/%s/raw"></iframe>`, link.ID)
		default:
			previewHTML = fmt.Sprintf(`<div style="padding:48px 20px;text-align:center;color:#94a3b8;">Ready to download (%s)</div>`, sizeStr)
		}
	}

	dlLabel := "Download File (" + sizeStr + ")"
	if fi.IsDir() {
		dlLabel = "Download Folder (.ZIP)"
	}

	metaBadges := fmt.Sprintf(`<span class="badge">%s</span>`, strings.ToUpper(link.Category))
	if link.MaxDownloads > 0 {
		metaBadges += fmt.Sprintf(`<span class="badge">%d / %d downloads</span>`, link.Downloads, link.MaxDownloads)
	}
	if link.ExpiresAt != nil {
		metaBadges += fmt.Sprintf(`<span class="badge">Expires %s</span>`, link.ExpiresAt.Format("Jan 02 15:04"))
	}

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s — Shared via awdex</title>
<style>
  * { box-sizing:border-box; }
  body { margin:0; min-height:100dvh; background:#090d16; color:#f8fafc; font-family:system-ui,-apple-system,sans-serif; display:flex; flex-direction:column; align-items:center; justify-content:center; padding:16px; }
  .share-card { background:#1e293b; border:1px solid #334155; border-radius:14px; width:100%%; max-width:920px; overflow:hidden; box-shadow:0 20px 25px -5px rgba(0,0,0,0.5); display:flex; flex-direction:column; }
  .share-header { padding:16px 20px; border-bottom:1px solid #334155; display:flex; flex-wrap:wrap; align-items:center; justify-content:space-between; gap:12px; }
  .title-group { min-width:0; flex:1; }
  .file-title { font-size:16px; font-weight:600; margin:0 0 6px; word-break:break-word; }
  .badges { display:flex; flex-wrap:wrap; gap:6px; }
  .badge { font-size:11px; background:#0f172a; color:#94a3b8; border:1px solid #334155; padding:2px 8px; border-radius:6px; }
  .dl-btn { display:inline-flex; align-items:center; gap:8px; background:#3b82f6; color:#fff; text-decoration:none; font-weight:600; font-size:14px; padding:10px 18px; border-radius:8px; white-space:nowrap; }
  .dl-btn:hover { background:#2563eb; }
  .preview-stage { background:#090d16; display:flex; align-items:center; justify-content:center; min-height:260px; max-height:75dvh; overflow:auto; }
  .media-player { width:100%%; max-height:72dvh; background:#000; outline:none; }
  .media-img { max-width:100%%; max-height:72dvh; object-fit:contain; display:block; }
  .pdf-frame { width:100%%; height:70dvh; border:none; }
  .folder-list { width:100%%; padding:12px 16px; display:flex; flex-direction:column; gap:6px; }
  .folder-item { display:flex; align-items:center; justify-content:space-between; gap:10px; padding:8px 12px; background:#1e293b; border:1px solid #334155; border-radius:8px; font-size:13px; }
  .fname { flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .fsize { color:#94a3b8; font-family:monospace; font-size:12px; }
  .fbtn { background:#334155; color:#f8fafc; text-decoration:none; padding:4px 10px; border-radius:6px; font-size:12px; }
  .fbtn:hover { background:#3b82f6; }
</style>
</head>
<body>
  <div class="share-card">
    <div class="share-header">
      <div class="title-group">
        <h1 class="file-title">%s</h1>
        <div class="badges">%s</div>
      </div>
      <a class="dl-btn" href="/s/%s/download">%s</a>
    </div>
    <div class="preview-stage">%s</div>
  </div>
</body></html>`, escapedName, escapedName, metaBadges, link.ID, dlLabel, previewHTML)
}
