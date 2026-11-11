package indexer

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/awdball/awdex/internal/config"
	"github.com/awdball/awdex/internal/db"
	"github.com/awdball/awdex/internal/model"
	"github.com/awdball/awdex/internal/thumb"
	"github.com/awdball/awdex/internal/util"
)

// State represents an explicit state in the filesystem indexer lifecycle state machine.
type State string

const (
	StateIdle     State = "idle"
	StateIndexing State = "indexing"
	StateReady    State = "ready"
	StateError    State = "error"
	StateStopping State = "stopping"
)

type Indexer struct {
	cfg      *config.Config
	database db.DB
	thumbs   *thumb.Manager

	mu             sync.RWMutex
	rootCtx        context.Context
	cancelFunc     context.CancelFunc
	state          State
	running        bool
	generation     uint64
	scannedCount   int64
	lastIndexedAt  *time.Time
	lastDurationMs int64
	lastError      string
}

func New(cfg *config.Config, database db.DB, thumbs *thumb.Manager) *Indexer {
	return &Indexer{
		cfg:      cfg,
		database: database,
		thumbs:   thumbs,
		rootCtx:  context.Background(),
		state:    StateIdle,
	}
}

// canTransition validates deterministic state machine transitions.
func canTransition(from, to State) bool {
	if from == to {
		return true
	}
	switch from {
	case StateIdle:
		return to == StateIndexing || to == StateStopping
	case StateIndexing:
		return to == StateReady || to == StateError || to == StateStopping || to == StateIdle
	case StateReady:
		return to == StateIndexing || to == StateStopping
	case StateError:
		return to == StateIndexing || to == StateStopping
	case StateStopping:
		return to == StateIdle
	default:
		return false
	}
}

func (idx *Indexer) State() string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return string(idx.state)
}

func (idx *Indexer) Status() model.IndexerStatus {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var lastAt *time.Time
	if idx.lastIndexedAt != nil {
		cp := *idx.lastIndexedAt
		lastAt = &cp
	}
	return model.IndexerStatus{
		State:          string(idx.state),
		Running:        idx.running,
		Generation:     idx.generation,
		ScannedCount:   idx.scannedCount,
		LastIndexedAt:  lastAt,
		LastDurationMs: idx.lastDurationMs,
		LastError:      idx.lastError,
	}
}

// Start begins background indexing loop and binds lifecycle to ctx.
func (idx *Indexer) Start(ctx context.Context) {
	idx.mu.Lock()
	idx.rootCtx = ctx
	idx.mu.Unlock()

	go func() {
		select {
		case <-ctx.Done():
			idx.Stop()
			return
		case <-time.After(1 * time.Second):
			idx.Trigger()
		}

		ticker := time.NewTicker(idx.cfg.IndexInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				idx.Stop()
				return
			case <-ticker.C:
				idx.Trigger()
			}
		}
	}()
}

// Stop cancels any in-flight crawl and transitions the indexer to stopping/idle.
func (idx *Indexer) Stop() {
	idx.mu.Lock()
	if canTransition(idx.state, StateStopping) {
		idx.state = StateStopping
	}
	cancel := idx.cancelFunc
	idx.cancelFunc = nil
	idx.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// Trigger atomically transitions the indexer into StateIndexing if not already running.
func (idx *Indexer) Trigger() bool {
	idx.mu.Lock()
	if idx.running || idx.state == StateStopping || !canTransition(idx.state, StateIndexing) {
		idx.mu.Unlock()
		return false
	}
	parentCtx := idx.rootCtx
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	if parentCtx.Err() != nil {
		idx.mu.Unlock()
		return false
	}

	crawlCtx, cancel := context.WithCancel(parentCtx)
	idx.state = StateIndexing
	idx.running = true
	idx.generation++
	idx.scannedCount = 0
	idx.lastError = ""
	idx.cancelFunc = cancel
	idx.mu.Unlock()

	go idx.runCrawl(crawlCtx, cancel)
	return true
}

func (idx *Indexer) runCrawl(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()

	start := time.Now()
	crawlStartMilli := start.UnixMilli()
	log.Printf("[indexer] starting background filesystem crawl on %s...", idx.cfg.RootDir)

	batch := make([]*model.FileItem, 0, 500)
	var count int64
	var flushErr error

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := idx.database.UpsertBatch(ctx, batch)
		batch = batch[:0]
		idx.mu.Lock()
		idx.scannedCount = count
		idx.mu.Unlock()
		return err
	}

	walkErr := filepath.WalkDir(idx.cfg.RootDir, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
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
				flushErr = err
				log.Printf("[indexer] flush error: %v", err)
			}
		}

		return nil
	})

	if ctx.Err() == nil {
		if err := flush(); err != nil {
			flushErr = err
			log.Printf("[indexer] final flush error: %v", err)
		}
	}

	duration := time.Since(start)
	now := time.Now()

	// Prune stale database entries if full walk succeeded without error or cancellation
	if walkErr == nil && flushErr == nil && ctx.Err() == nil {
		if pruned, pruneErr := idx.database.PruneStale(ctx, crawlStartMilli); pruneErr != nil {
			log.Printf("[indexer] prune stale error: %v", pruneErr)
		} else if pruned > 0 {
			log.Printf("[indexer] pruned %d deleted entries from index", pruned)
		}
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.running = false
	idx.cancelFunc = nil
	idx.scannedCount = count
	idx.lastDurationMs = duration.Milliseconds()

	if ctx.Err() != nil {
		log.Printf("[indexer] crawl cancelled after %v (%d items)", duration, count)
		if idx.state == StateStopping {
			idx.state = StateIdle
		} else if canTransition(idx.state, StateIdle) {
			idx.state = StateIdle
		}
		return
	}

	finalErr := walkErr
	if finalErr == nil {
		finalErr = flushErr
	}

	if finalErr != nil {
		log.Printf("[indexer] walk finished with error: %v", finalErr)
		idx.lastError = finalErr.Error()
		if canTransition(idx.state, StateError) {
			idx.state = StateError
		}
	} else {
		log.Printf("[indexer] completed! Indexed %d items in %v", count, duration)
		idx.lastIndexedAt = &now
		idx.lastError = ""
		if canTransition(idx.state, StateReady) {
			idx.state = StateReady
		}
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
