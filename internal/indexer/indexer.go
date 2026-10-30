package indexer

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/awdball/awdex/internal/config"
	"github.com/awdball/awdex/internal/db"
	"github.com/awdball/awdex/internal/model"
	"github.com/awdball/awdex/internal/thumb"
	"github.com/awdball/awdex/internal/util"
)

type Indexer struct {
	cfg        *config.Config
	database   db.DB
	thumbs     *thumb.Manager
	state      string
	stateLock  sync.RWMutex
	cancelFunc context.CancelFunc
	running    int32
}

func New(cfg *config.Config, database db.DB, thumbs *thumb.Manager) *Indexer {
	return &Indexer{
		cfg:      cfg,
		database: database,
		thumbs:   thumbs,
		state:    "idle",
	}
}

func (idx *Indexer) State() string {
	idx.stateLock.RLock()
	defer idx.stateLock.RUnlock()
	return idx.state
}

func (idx *Indexer) setState(s string) {
	idx.stateLock.Lock()
	idx.state = s
	idx.stateLock.Unlock()
}

// Start begins background indexing loop
func (idx *Indexer) Start(ctx context.Context) {
	go func() {
		// Run initial crawl after a short startup delay
		time.Sleep(1 * time.Second)
		idx.Trigger()

		ticker := time.NewTicker(idx.cfg.IndexInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				idx.Trigger()
			}
		}
	}()
}

// Trigger triggers a re-index if not already running
func (idx *Indexer) Trigger() {
	if !atomic.CompareAndSwapInt32(&idx.running, 0, 1) {
		return
	}

	go func() {
		defer atomic.StoreInt32(&idx.running, 0)
		idx.runCrawl()
	}()
}

func (idx *Indexer) runCrawl() {
	idx.setState("indexing")
	start := time.Now()
	log.Printf("[indexer] starting background filesystem crawl on %s...", idx.cfg.RootDir)

	ctx, cancel := context.WithCancel(context.Background())
	idx.cancelFunc = cancel
	defer cancel()

	batch := make([]*model.FileItem, 0, 500)
	var count int64

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := idx.database.UpsertBatch(ctx, batch)
		batch = batch[:0]
		return err
	}

	err := filepath.WalkDir(idx.cfg.RootDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // continue on permission error
		}

		name := d.Name()
		// Skip hidden files, system dirs, and awdex asset cache
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "$RECYCLE.BIN" || name == "awdex" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(idx.cfg.RootDir, p)
		if err != nil {
			return nil
		}

		cleanRel := util.CleanRelativePath(filepath.ToSlash(rel))
		if cleanRel == "/" {
			return nil // Root dir itself
		}

		isDir := d.IsDir()
		var size int64
		var modTime time.Time

		info, err := d.Info()
		if err == nil {
			size = info.Size()
			modTime = info.ModTime()
		} else {
			modTime = time.Now()
		}

		cat, mimeType, ext := util.DetectCategory(name, isDir)
		hasThumb := idx.thumbs.CanThumbnail(cat, ext)

		item := &model.FileItem{
			Name:      name,
			Path:      cleanRel,
			IsDir:     isDir,
			Size:      size,
			ModTime:   modTime,
			MimeType:  mimeType,
			Category:  cat,
			Extension: ext,
			HasThumb:  hasThumb,
		}

		batch = append(batch, item)
		count++

		if len(batch) >= 500 {
			if err := flush(); err != nil {
				log.Printf("[indexer] flush error: %v", err)
			}
		}

		return nil
	})

	if err := flush(); err != nil {
		log.Printf("[indexer] final flush error: %v", err)
	}

	if err != nil {
		log.Printf("[indexer] walk finished with error: %v", err)
		idx.setState("error")
	} else {
		log.Printf("[indexer] completed! Indexed %d items in %v", count, time.Since(start))
		idx.setState("ready")
	}
}

// IndexSingleItem adds or updates a single item immediately (e.g. after upload)
func (idx *Indexer) IndexSingleItem(relPath string) {
	fullPath := filepath.Join(idx.cfg.RootDir, filepath.FromSlash(strings.TrimPrefix(relPath, "/")))
	fi, err := os.Stat(fullPath)
	if err != nil {
		return
	}

	cat, mimeType, ext := util.DetectCategory(fi.Name(), fi.IsDir())
	item := &model.FileItem{
		Name:      fi.Name(),
		Path:      util.CleanRelativePath(relPath),
		IsDir:     fi.IsDir(),
		Size:      fi.Size(),
		ModTime:   fi.ModTime(),
		MimeType:  mimeType,
		Category:  cat,
		Extension: ext,
		HasThumb:  idx.thumbs.CanThumbnail(cat, ext),
	}

	_ = idx.database.UpsertBatch(context.Background(), []*model.FileItem{item})
}
