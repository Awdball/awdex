package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/awdball/awdex/internal/config"
	"github.com/awdball/awdex/internal/db"
	"github.com/awdball/awdex/internal/model"
	"github.com/awdball/awdex/internal/thumb"
)

func TestIndexerStateMachineAndPruning(t *testing.T) {
	rootDir := t.TempDir()
	dataDir := t.TempDir()

	fileA := filepath.Join(rootDir, "alpha.txt")
	fileB := filepath.Join(rootDir, "beta.txt")
	if err := os.WriteFile(fileA, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		RootDir:          rootDir,
		DataDir:          dataDir,
		EnableThumbnails: false,
		IndexInterval:    10 * time.Minute,
	}

	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	thumbs := thumb.NewManager(cfg)
	idx := New(cfg, database, thumbs)

	if st := idx.Status(); st.State != string(StateIdle) || st.Running {
		t.Fatalf("expected initial state idle, got %+v", st)
	}

	// Trigger first crawl
	if !idx.Trigger() {
		t.Fatalf("expected Trigger() to succeed from idle")
	}

	// Wait for ready
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st := idx.Status()
		if st.State == string(StateReady) && !st.Running {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	st := idx.Status()
	if st.State != string(StateReady) || st.ScannedCount != 2 || st.Generation != 1 {
		t.Fatalf("unexpected indexer status after first crawl: %+v", st)
	}

	// Delete beta.txt from disk and re-trigger crawl; verify PruneStale removes beta.txt
	time.Sleep(5 * time.Millisecond)
	if err := os.Remove(fileB); err != nil {
		t.Fatal(err)
	}

	if !idx.Trigger() {
		t.Fatalf("expected second Trigger() to succeed from ready")
	}
	for time.Now().Before(deadline) {
		st = idx.Status()
		if st.Generation == 2 && st.State == string(StateReady) && !st.Running {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	items, total, err := database.Search(context.Background(), model.SearchRequest{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].Name != "alpha.txt" {
		t.Fatalf("expected stale deleted file beta.txt to be pruned, got total=%d items=%+v", total, items)
	}
}
