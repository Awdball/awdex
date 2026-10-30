package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/awdball/awdex/internal/util"
)

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		http.Error(w, "Server is in read-only mode", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 128 MB max memory buffer for headers, rest streams to disk
	reader, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "Failed to read multipart upload: "+err.Error(), http.StatusBadRequest)
		return
	}

	destDirRel := util.CleanRelativePath(r.URL.Query().Get("path"))
	destFullDir := filepath.Join(s.cfg.RootDir, filepath.FromSlash(strings.TrimPrefix(destDirRel, "/")))

	if err := os.MkdirAll(destFullDir, 0755); err != nil {
		http.Error(w, "Failed to create destination directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var uploadedFiles []string

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Error reading upload stream: "+err.Error(), http.StatusInternalServerError)
			return
		}

		filename := part.FileName()
		if filename == "" {
			continue
		}

		// Sanitize filename
		filename = filepath.Base(filepath.Clean(filename))
		targetPath := filepath.Join(destFullDir, filename)

		dst, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create file %s: %v", filename, err), http.StatusInternalServerError)
			return
		}

		_, copyErr := io.Copy(dst, part)
		dst.Close()

		if copyErr != nil {
			http.Error(w, fmt.Sprintf("Failed to write file %s: %v", filename, copyErr), http.StatusInternalServerError)
			return
		}

		fileRelPath := filepath.ToSlash(filepath.Join(destDirRel, filename))
		s.indexer.IndexSingleItem(fileRelPath)
		uploadedFiles = append(uploadedFiles, filename)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"uploaded": uploadedFiles,
	})
}

func (s *Server) handleMkdir(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		http.Error(w, "Server is in read-only mode", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	name := filepath.Base(filepath.Clean(payload.Name))
	if name == "" || name == "." || name == "/" {
		http.Error(w, "Invalid directory name", http.StatusBadRequest)
		return
	}

	parentRel := util.CleanRelativePath(payload.Path)
	fullDir := filepath.Join(s.cfg.RootDir, filepath.FromSlash(strings.TrimPrefix(parentRel, "/")), name)

	if err := os.MkdirAll(fullDir, 0755); err != nil {
		http.Error(w, "Failed to create directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	newRel := filepath.ToSlash(filepath.Join(parentRel, name))
	s.indexer.IndexSingleItem(newRel)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"path":    newRel,
	})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		http.Error(w, "Server is in read-only mode", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		Path string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	relPath := util.CleanRelativePath(payload.Path)
	if relPath == "/" {
		http.Error(w, "Cannot delete root directory", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(s.cfg.RootDir, filepath.FromSlash(strings.TrimPrefix(relPath, "/")))
	fi, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	if fi.IsDir() {
		if err := os.RemoveAll(fullPath); err != nil {
			http.Error(w, "Failed to delete directory: "+err.Error(), http.StatusInternalServerError)
			return
		}
		_ = s.database.DeleteDirPrefix(r.Context(), relPath)
	} else {
		if err := os.Remove(fullPath); err != nil {
			http.Error(w, "Failed to delete file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		_ = s.database.DeletePath(r.Context(), relPath)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}
