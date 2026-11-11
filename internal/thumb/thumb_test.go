package thumb

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/awdball/awdex/internal/config"
)

func TestThumbNoCoverNegativeCacheAndSingleFlight(t *testing.T) {
	rootDir := t.TempDir()
	dataDir := t.TempDir()

	// Create a dummy audio file with no cover art and no sibling cover image
	audioPath := filepath.Join(rootDir, "track.mp3")
	if err := os.WriteFile(audioPath, []byte("not-a-valid-mp3"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		RootDir:          rootDir,
		DataDir:          dataDir,
		EnableThumbnails: true,
	}
	mgr := NewManager(cfg)

	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := mgr.GetThumbnail("/track.mp3")
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if !errors.Is(err, ErrNoCoverArt) {
			t.Fatalf("goroutine %d: expected ErrNoCoverArt, got %v", i, err)
		}
	}

	// Second call should hit negative cache immediately and return ErrNoCoverArt
	if _, err := mgr.GetThumbnail("/track.mp3"); !errors.Is(err, ErrNoCoverArt) {
		t.Fatalf("expected cached ErrNoCoverArt on subsequent call, got %v", err)
	}
}
