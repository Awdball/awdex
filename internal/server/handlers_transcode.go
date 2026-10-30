package server

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/awdball/awdex/internal/util"
)

func (s *Server) handleTranscode(w http.ResponseWriter, r *http.Request) {
	if !s.thumbs.HasFFmpeg() {
		http.Error(w, "FFmpeg is not available on this server", http.StatusNotImplemented)
		return
	}

	relPath := util.CleanRelativePath(r.URL.Query().Get("path"))
	fullPath := filepath.Join(s.cfg.RootDir, filepath.FromSlash(strings.TrimPrefix(relPath, "/")))

	fi, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Video not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Permission denied", http.StatusForbidden)
		return
	}

	if fi.IsDir() {
		http.Error(w, "Cannot transcode a directory", http.StatusBadRequest)
		return
	}

	// Build FFmpeg command for on-the-fly fragmented MP4 streaming
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
	}

	// Fast seek if specified
	if ss := strings.TrimSpace(r.URL.Query().Get("ss")); ss != "" && ss != "0" {
		args = append(args, "-ss", ss)
	}

	args = append(args,
		"-i", fullPath,
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-crf", "23",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ac", "2",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4",
		"pipe:1",
	)

	cmd := exec.CommandContext(r.Context(), "ffmpeg", args...)
	cmd.Stdout = w

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if err := cmd.Start(); err != nil {
		log.Printf("[transcode] failed to start ffmpeg: %v", err)
		http.Error(w, "Transcoding failed to start", http.StatusInternalServerError)
		return
	}

	if err := cmd.Wait(); err != nil {
		// Context cancellation (user closed browser tab or modal) is normal
		if r.Context().Err() == nil {
			log.Printf("[transcode] ffmpeg exited with error: %v", err)
		}
	}
}
