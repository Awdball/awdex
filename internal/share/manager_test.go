package share

import (
	"sync"
	"testing"

	"github.com/awdball/awdex/internal/config"
)

func TestShareAtomicDownloadClaimAndSettingsState(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataDir:        dataDir,
		SharePort:      8081,
		ShareProvider:  "custom",
		SharePublicURL: "https://share.example.com",
	}

	mgr := NewManager(cfg)

	st, _ := mgr.Status()
	if st.State != TunnelStateOnline || !st.Running {
		t.Fatalf("expected custom provider with URL to be online, got %+v", st)
	}

	// Updating settings with empty provider should NOT reset provider or stop tunnel
	updated := mgr.UpdateSettings("", "https://share2.example.com/", "")
	if updated.Provider != "custom" || updated.PublicBaseURL != "https://share2.example.com" {
		t.Fatalf("unexpected settings after partial update: %+v", updated)
	}

	// Create a burn-after-reading share (MaxDownloads = 1)
	link, err := mgr.CreateShare("/secret.txt", "secret.txt", false, 128, "text", 1, 1, "")
	if err != nil {
		t.Fatalf("CreateShare failed: %v", err)
	}
	if link.State != ShareStateActive {
		t.Fatalf("expected new share state active, got %s", link.State)
	}

	// Concurrently attempt 10 downloads on a MaxDownloads=1 link; exactly 1 must succeed
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := mgr.ClaimDownload(link.ID); ok {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected exactly 1 atomic download claim to succeed for MaxDownloads=1, got %d", successCount)
	}

	// Subsequent GetShare must return false since quota is exhausted
	if _, ok := mgr.GetShare(link.ID); ok {
		t.Fatalf("expected exhausted share to no longer be active")
	}
}
