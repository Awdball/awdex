package server

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/awdball/awdex/internal/config"
	"github.com/awdball/awdex/internal/db"
	"github.com/awdball/awdex/internal/indexer"
	"github.com/awdball/awdex/internal/thumb"
)

type Server struct {
	cfg        *config.Config
	database   db.DB
	thumbs     *thumb.Manager
	indexer    *indexer.Indexer
	httpServer *http.Server
	webFS      fs.FS
}

func New(cfg *config.Config, database db.DB, thumbs *thumb.Manager, idx *indexer.Indexer, webFS fs.FS) *Server {
	return &Server{
		cfg:      cfg,
		database: database,
		thumbs:   thumbs,
		indexer:  idx,
		webFS:    webFS,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/browse", s.handleBrowse)
	mux.HandleFunc("/api/tree", s.handleTree)
	mux.HandleFunc("/api/thumb", s.handleThumb)
	mux.HandleFunc("/api/scrub", s.handleScrub)
	mux.HandleFunc("/api/transcode", s.handleTranscode)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/info", s.handleInfo)
	mux.HandleFunc("/api/upload", s.handleUpload)
	mux.HandleFunc("/api/mkdir", s.handleMkdir)
	mux.HandleFunc("/api/delete", s.handleDelete)
	mux.HandleFunc("/api/archive", s.handleArchiveInspect)

	// Raw file streaming with range support
	mux.HandleFunc("/raw/", s.handleRawFile)

	// Frontend SPA handler
	if s.webFS != nil {
		distSub, err := fs.Sub(s.webFS, "dist")
		if err == nil {
			fileServer := http.FileServer(http.FS(distSub))
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				path := strings.TrimPrefix(r.URL.Path, "/")
				if path == "" {
					fileServer.ServeHTTP(w, r)
					return
				}
				// If file exists in assets, serve it
				if f, err := distSub.Open(path); err == nil {
					f.Close()
					fileServer.ServeHTTP(w, r)
					return
				}
				// Otherwise serve index.html for SPA router
				indexFile, err := distSub.Open("index.html")
				if err == nil {
					defer indexFile.Close()
					stat, _ := indexFile.Stat()
					http.ServeContent(w, r, "index.html", stat.ModTime(), indexFile.(io.ReadSeeker))
					return
				}
				http.NotFound(w, r)
			})
		}
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body><h1>awdex backend running</h1><p>Frontend assets not embedded. Running API only.</p></body></html>")
		})
	}

	handler := Logger(SecurityAndCORS(mux))

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  120 * time.Minute, // Long timeout for large file uploads
		WriteTimeout: 120 * time.Minute, // Long timeout for large file downloads
	}

	log.Printf("[awdex] server starting at http://%s (root: %s, data: %s, db: %s)", addr, s.cfg.RootDir, s.cfg.DataDir, s.database.Type())
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
