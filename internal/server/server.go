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
	"github.com/awdball/awdex/internal/share"
	"github.com/awdball/awdex/internal/thumb"
)

type Server struct {
	cfg         *config.Config
	database    db.DB
	thumbs      *thumb.Manager
	indexer     *indexer.Indexer
	shares      *share.Manager
	httpServer  *http.Server
	shareServer *http.Server
	webFS       fs.FS
}

func New(cfg *config.Config, database db.DB, thumbs *thumb.Manager, idx *indexer.Indexer, webFS fs.FS) *Server {
	return &Server{
		cfg:      cfg,
		database: database,
		thumbs:   thumbs,
		indexer:  idx,
		shares:   share.NewManager(cfg),
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
	mux.HandleFunc("/api/shares", s.handleSharesAPI)
	mux.HandleFunc("/api/shares/config", s.handleSharesConfigAPI)

	// Public share endpoint also mounted on main port for LAN convenience
	mux.HandleFunc("/s/", s.handlePublicShare)

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
				if f, err := distSub.Open(path); err == nil {
					f.Close()
					fileServer.ServeHTTP(w, r)
					return
				}
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

	// Start isolated Share-Only HTTP Server on SharePort (default 8081)
	if s.cfg.SharePort > 0 && s.cfg.SharePort != s.cfg.Port {
		shareMux := http.NewServeMux()
		shareMux.HandleFunc("/s/", s.handlePublicShare)
		shareAddr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.SharePort)
		s.shareServer = &http.Server{
			Addr:         shareAddr,
			Handler:      Logger(SecurityAndCORS(shareMux)),
			ReadTimeout:  120 * time.Minute,
			WriteTimeout: 120 * time.Minute,
		}
		go func() {
			log.Printf("[awdex] isolated share-only listener starting at http://%s/s/<token>", shareAddr)
			if err := s.shareServer.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
				log.Printf("[awdex] share server error: %v", err)
			}
		}()
	}

	handler := Logger(SecurityAndCORS(mux))

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  120 * time.Minute,
		WriteTimeout: 120 * time.Minute,
	}

	log.Printf("[awdex] server starting at http://%s (root: %s, data: %s, db: %s)", addr, s.cfg.RootDir, s.cfg.DataDir, s.database.Type())
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.shares.StopTunnel()
	if s.shareServer != nil {
		_ = s.shareServer.Shutdown(ctx)
	}
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
