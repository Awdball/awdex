package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/awdball/awdex/internal/model"
	"github.com/awdball/awdex/internal/util"
)

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	relPath := util.CleanRelativePath(r.URL.Query().Get("path"))
	fullPath := filepath.Join(s.cfg.RootDir, filepath.FromSlash(strings.TrimPrefix(relPath, "/")))

	fi, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Directory not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Permission denied", http.StatusForbidden)
		return
	}

	if !fi.IsDir() {
		http.Error(w, "Path is a file, not a directory", http.StatusBadRequest)
		return
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		http.Error(w, "Failed to read directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var items []*model.FileItem
	var totalDirs, totalFiles int
	var totalBytes int64

	for _, entry := range entries {
		name := entry.Name()
		// Hide dotfiles and internal awdex asset folder
		if strings.HasPrefix(name, ".") || name == "awdex" {
			continue
		}

		itemRelPath := filepath.ToSlash(filepath.Join(relPath, name))
		isDir := entry.IsDir()
		var size int64
		info, err := entry.Info()
		if err == nil {
			size = info.Size()
		}

		cat, mimeType, ext := util.DetectCategory(name, isDir)
		hasThumb := s.thumbs.CanThumbnail(cat, ext)

		item := &model.FileItem{
			Name:      name,
			Path:      itemRelPath,
			IsDir:     isDir,
			Size:      size,
			Category:  cat,
			MimeType:  mimeType,
			Extension: ext,
			HasThumb:  hasThumb,
		}

		if info != nil {
			item.ModTime = info.ModTime()
		}

		if isDir {
			totalDirs++
		} else {
			totalFiles++
			totalBytes += size
		}

		if hasThumb {
			item.ThumbURL = "/api/thumb?path=" + itemRelPath
			if cat == util.CatVideo && s.cfg.EnableVideoScrub && s.thumbs.HasFFmpeg() {
				item.ScrubURL = "/api/scrub?path=" + itemRelPath
			}
		}

		items = append(items, item)
	}

	// Sort directories first, then alphabetical case-insensitive
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	parentPath := "/"
	if relPath != "/" {
		parentPath = filepath.ToSlash(filepath.Dir(relPath))
	}

	resp := model.BrowseResponse{
		Path:        relPath,
		ParentPath:  parentPath,
		Breadcrumbs: buildBreadcrumbs(relPath),
		Items:       items,
		TotalCount:  len(items),
		TotalDirs:   totalDirs,
		TotalFiles:  totalFiles,
		TotalBytes:  totalBytes,
		ReadOnly:    s.cfg.ReadOnly,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	relPath := util.CleanRelativePath(r.URL.Query().Get("path"))
	fullPath := filepath.Join(s.cfg.RootDir, filepath.FromSlash(strings.TrimPrefix(relPath, "/")))

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		http.Error(w, "Cannot read path", http.StatusNotFound)
		return
	}

	var nodes []*model.TreeNode
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "awdex" {
			continue
		}

		name := entry.Name()
		subRelPath := filepath.ToSlash(filepath.Join(relPath, name))
		subFullPath := filepath.Join(fullPath, name)

		hasChildren := false
		if subF, err := os.Open(subFullPath); err == nil {
			if subEntries, err := subF.ReadDir(20); err == nil {
				for _, sub := range subEntries {
					if sub.IsDir() && !strings.HasPrefix(sub.Name(), ".") {
						hasChildren = true
						break
					}
				}
			}
			subF.Close()
		}

		nodes = append(nodes, &model.TreeNode{
			Name:        name,
			Path:        subRelPath,
			HasChildren: hasChildren,
		})
	}

	sort.Slice(nodes, func(i, j int) bool {
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(nodes)
}

func buildBreadcrumbs(relPath string) []model.Crumb {
	var crumbs []model.Crumb
	crumbs = append(crumbs, model.Crumb{Name: "Root", Path: "/"})

	if relPath == "/" || relPath == "" {
		return crumbs
	}

	parts := strings.Split(strings.Trim(relPath, "/"), "/")
	accum := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		accum += "/" + part
		crumbs = append(crumbs, model.Crumb{
			Name: part,
			Path: accum,
		})
	}

	return crumbs
}
