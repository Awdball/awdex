package server

import (
	"net/http"
	"os"

	"github.com/awdball/awdex/internal/util"
)

func (s *Server) handleThumb(w http.ResponseWriter, r *http.Request) {
	relPath := util.CleanRelativePath(r.URL.Query().Get("path"))
	if relPath == "/" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	thumbPath, err := s.thumbs.GetThumbnail(relPath)
	if err != nil {
		http.Error(w, "Failed to generate thumbnail: "+err.Error(), http.StatusInternalServerError)
		return
	}

	fi, err := os.Stat(thumbPath)
	if err != nil {
		http.Error(w, "Thumbnail not found", http.StatusNotFound)
		return
	}

	file, err := os.Open(thumbPath)
	if err != nil {
		http.Error(w, "Cannot open thumbnail", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=604800, stale-while-revalidate=2592000")
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), file)
}

func (s *Server) handleScrub(w http.ResponseWriter, r *http.Request) {
	relPath := util.CleanRelativePath(r.URL.Query().Get("path"))
	if relPath == "/" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	scrubPath, err := s.thumbs.GetVideoScrub(relPath)
	if err != nil {
		http.Error(w, "Failed to generate video scrub: "+err.Error(), http.StatusInternalServerError)
		return
	}

	fi, err := os.Stat(scrubPath)
	if err != nil {
		http.Error(w, "Video scrub not found", http.StatusNotFound)
		return
	}

	file, err := os.Open(scrubPath)
	if err != nil {
		http.Error(w, "Cannot open video scrub", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Cache-Control", "public, max-age=604800, stale-while-revalidate=2592000")
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), file)
}
