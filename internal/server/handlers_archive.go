package server

import (
	"archive/zip"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/awdball/awdex/internal/model"
	"github.com/awdball/awdex/internal/util"
)

func (s *Server) handleArchiveInspect(w http.ResponseWriter, r *http.Request) {
	relPath := util.CleanRelativePath(r.URL.Query().Get("path"))
	fullPath := filepath.Join(s.cfg.RootDir, filepath.FromSlash(strings.TrimPrefix(relPath, "/")))

	fi, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	if fi.IsDir() || !strings.HasSuffix(strings.ToLower(fi.Name()), ".zip") {
		http.Error(w, "Only zip archives can be inspected", http.StatusBadRequest)
		return
	}

	reader, err := zip.OpenReader(fullPath)
	if err != nil {
		http.Error(w, "Failed to read zip archive: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer reader.Close()

	var entries []model.ArchiveEntry
	for _, f := range reader.File {
		entries = append(entries, model.ArchiveEntry{
			Name:    f.Name,
			IsDir:   f.FileInfo().IsDir(),
			Size:    f.UncompressedSize64,
			ModTime: f.Modified,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(entries)
}
