package server

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/awdball/awdex/internal/util"
)

type flushWriter struct {
	w http.ResponseWriter
	f http.Flusher
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if fw.f != nil {
		fw.f.Flush()
	}
	return n, err
}

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

	s.streamFFmpegMP4(w, r, fullPath, strings.TrimSpace(r.URL.Query().Get("ss")))
}

func (s *Server) streamFFmpegMP4(w http.ResponseWriter, r *http.Request, fullPath string, ss string) {
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Accel-Buffering", "no")

	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Build FFmpeg command for low-latency, monotonically synchronized fragmented MP4 streaming
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
	}

	if ss != "" && ss != "0" {
		args = append(args, "-ss", ss)
	}

	args = append(args,
		"-i", fullPath,
		"-map", "0:v:0",
		"-map", "0:a:0?",
		"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2,fps=30,setpts=N/(30*TB)",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-profile:v", "baseline",
		"-crf", "23",
		"-pix_fmt", "yuv420p",
		"-g", "30",
		"-keyint_min", "30",
		"-sc_threshold", "0",
		"-af", "aresample=44100:async=1:first_pts=0,asetpts=(N+1024)/SR/TB",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ac", "2",
		"-max_muxing_queue_size", "1024",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof+separate_moof",
		"-frag_duration", "500000",
		"-f", "mp4",
		"pipe:1",
	)

	cmd := exec.CommandContext(r.Context(), "ffmpeg", args...)

	var out io.Writer = w
	if flusher, ok := w.(http.Flusher); ok {
		out = &flushWriter{w: w, f: flusher}
	}
	cmd.Stdout = out

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		log.Printf("[transcode] failed to start ffmpeg for %s: %v", fullPath, err)
		http.Error(w, "Transcoding failed to start", http.StatusInternalServerError)
		return
	}

	if err := cmd.Wait(); err != nil {
		// Context cancellation (user closed modal or switched video) is normal
		if r.Context().Err() == nil {
			log.Printf("[transcode] ffmpeg exited with error for %s: %v (stderr: %s)", fullPath, err, strings.TrimSpace(stderr.String()))
		}
	}
}
