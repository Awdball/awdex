package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/awdball/awdex/internal/util"
)

func (s *Server) handleRawFile(w http.ResponseWriter, r *http.Request) {
	relPath := util.CleanRelativePath(strings.TrimPrefix(r.URL.Path, "/raw"))
	fullPath := filepath.Join(s.cfg.RootDir, filepath.FromSlash(strings.TrimPrefix(relPath, "/")))

	fi, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Permission denied", http.StatusForbidden)
		return
	}

	if fi.IsDir() {
		http.Error(w, "Cannot stream a directory", http.StatusBadRequest)
		return
	}

	file, err := os.Open(fullPath)
	if err != nil {
		http.Error(w, "Cannot open file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	_, mimeType, _ := util.DetectCategory(fi.Name(), false)
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Accept-Ranges", "bytes")

	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(fullPath)))
	} else {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filepath.Base(fullPath)))
	}

	http.ServeContent(w, r, fi.Name(), fi.ModTime(), file)
}
