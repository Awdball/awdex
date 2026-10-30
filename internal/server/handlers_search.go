package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/awdball/awdex/internal/model"
	"github.com/awdball/awdex/internal/util"
)

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	category := r.URL.Query().Get("category")
	pathScope := util.CleanRelativePath(r.URL.Query().Get("path"))

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	req := model.SearchRequest{
		Query:    q,
		Category: category,
		Path:     pathScope,
		Limit:    limit,
		Offset:   offset,
	}

	items, total, err := s.database.Search(r.Context(), req)
	if err != nil {
		http.Error(w, "Search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	for _, it := range items {
		if it.HasThumb {
			it.ThumbURL = "/api/thumb?path=" + url.QueryEscape(it.Path)
			if it.Category == util.CatVideo && s.cfg.EnableVideoScrub && s.thumbs.HasFFmpeg() {
				it.ScrubURL = "/api/scrub?path=" + url.QueryEscape(it.Path)
			}
		}
	}

	resp := model.SearchResponse{
		Items:      items,
		TotalCount: total,
		Query:      q,
		Limit:      req.Limit,
		Offset:     req.Offset,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	totalFiles, totalDirs, totalBytes, _ := s.database.GetStats(r.Context())

	info := model.ServerInfo{
		Title:          s.cfg.Title,
		Version:        "1.0.0",
		ReadOnly:       s.cfg.ReadOnly,
		DatabaseType:   s.database.Type(),
		IndexedFiles:   totalFiles,
		IndexedDirs:    totalDirs,
		IndexedBytes:   totalBytes,
		IndexState:     s.indexer.State(),
		FFmpegPresent:  s.thumbs.HasFFmpeg(),
		PopplerPresent: s.thumbs.HasPoppler(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}
