package share

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/awdball/awdex/internal/config"
)

type ShareLink struct {
	ID           string     `json:"id"`
	Path         string     `json:"path"`
	Name         string     `json:"name"`
	IsDir        bool       `json:"is_dir"`
	Size         int64      `json:"size"`
	Category     string     `json:"category"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	MaxDownloads int        `json:"max_downloads"`
	Downloads    int        `json:"downloads"`
	PasswordHash string     `json:"password_hash,omitempty"`
	HasPassword  bool       `json:"has_password"`
	Provider     string     `json:"provider"`
	PublicURL    string     `json:"public_url"`
}

type TunnelSettings struct {
	Provider      string `json:"provider"`        // "cloudflared", "zrok", "custom"
	PublicBaseURL string `json:"public_base_url"` // For "custom" (e.g. Tailscale Funnel)
	ZrokToken     string `json:"zrok_token,omitempty"`
	HasZrokToken  bool   `json:"has_zrok_token"`
}

type TunnelStatus struct {
	Running       bool   `json:"running"`
	Provider      string `json:"provider"`
	PublicBaseURL string `json:"public_base_url"`
	SharePort     int    `json:"share_port"`
	Error         string `json:"error,omitempty"`
}

type persistedData struct {
	Settings TunnelSettings       `json:"settings"`
	Shares   map[string]*ShareLink `json:"shares"`
}

type Manager struct {
	cfg        *config.Config
	mu         sync.RWMutex
	shares     map[string]*ShareLink
	settings   TunnelSettings
	storePath  string

	tunnelMu     sync.Mutex
	tunnelCmd    *exec.Cmd
	tunnelCancel context.CancelFunc
	tunnelURL    string
	tunnelErr    string
	tunnelProv   string
}

var (
	cfURLRegex   = regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)
	zrokURLRegex = regexp.MustCompile(`https://[a-zA-Z0-9-]+\.share\.zrok\.io`)
)

func NewManager(cfg *config.Config) *Manager {
	m := &Manager{
		cfg:       cfg,
		shares:    make(map[string]*ShareLink),
		storePath: filepath.Join(cfg.DataDir, "shares.json"),
		settings: TunnelSettings{
			Provider:      cfg.ShareProvider,
			PublicBaseURL: cfg.SharePublicURL,
			ZrokToken:     cfg.ZrokToken,
		},
	}
	if m.settings.Provider == "" {
		m.settings.Provider = "cloudflared"
	}
	m.load()
	go m.cleanupLoop()
	return m
}

func (m *Manager) load() {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.storePath)
	if err != nil {
		return
	}
	var pd persistedData
	if err := json.Unmarshal(data, &pd); err == nil {
		if pd.Shares != nil {
			m.shares = pd.Shares
		}
		if pd.Settings.Provider != "" {
			m.settings.Provider = pd.Settings.Provider
		}
		if pd.Settings.PublicBaseURL != "" {
			m.settings.PublicBaseURL = pd.Settings.PublicBaseURL
		}
		if pd.Settings.ZrokToken != "" {
			m.settings.ZrokToken = pd.Settings.ZrokToken
		}
	}
}

func (m *Manager) saveLocked() {
	pd := persistedData{
		Settings: m.settings,
		Shares:   m.shares,
	}
	b, err := json.MarshalIndent(pd, "", "  ")
	if err == nil {
		_ = os.WriteFile(m.storePath, b, 0600)
	}
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		m.PruneExpired()
	}
}

func (m *Manager) PruneExpired() {
	m.mu.Lock()
	now := time.Now()
	changed := false
	for id, s := range m.shares {
		if s.ExpiresAt != nil && now.After(*s.ExpiresAt) {
			delete(m.shares, id)
			changed = true
		} else if s.MaxDownloads > 0 && s.Downloads >= s.MaxDownloads {
			delete(m.shares, id)
			changed = true
		}
	}
	remaining := len(m.shares)
	if changed {
		m.saveLocked()
	}
	m.mu.Unlock()

	if changed && remaining == 0 {
		m.StopTunnel()
	}
}

func generateToken(length int) string {
	const chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

func HashPassword(pw string) string {
	if pw == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("awdex-share-salt:" + pw))
	return hex.EncodeToString(sum[:])
}

func (m *Manager) UpdateSettings(prov, publicBaseURL, zrokToken string) TunnelSettings {
	m.mu.Lock()
	oldProv := m.settings.Provider
	if prov != "" {
		m.settings.Provider = prov
	}
	m.settings.PublicBaseURL = strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if zrokToken != "" {
		m.settings.ZrokToken = strings.TrimSpace(zrokToken)
	}
	m.saveLocked()
	res := m.settings
	res.HasZrokToken = res.ZrokToken != ""
	res.ZrokToken = ""
	m.mu.Unlock()

	if oldProv != prov {
		m.StopTunnel()
	}
	return res
}

func (m *Manager) CreateShare(relPath, name string, isDir bool, size int64, category string, expiresInHours float64, maxDownloads int, password string) (*ShareLink, error) {
	m.PruneExpired()

	m.mu.RLock()
	prov := m.settings.Provider
	m.mu.RUnlock()

	baseURL, err := m.EnsureTunnel()
	if err != nil {
		log.Printf("[share] tunnel warning (%s): %v", prov, err)
	}

	token := generateToken(12)
	now := time.Now()
	var exp *time.Time
	if expiresInHours > 0 {
		t := now.Add(time.Duration(expiresInHours * float64(time.Hour)))
		exp = &t
	}

	pwHash := HashPassword(password)
	link := &ShareLink{
		ID:           token,
		Path:         relPath,
		Name:         name,
		IsDir:        isDir,
		Size:         size,
		Category:     category,
		CreatedAt:    now,
		ExpiresAt:    exp,
		MaxDownloads: maxDownloads,
		Downloads:    0,
		PasswordHash: pwHash,
		HasPassword:  pwHash != "",
		Provider:     prov,
	}
	if baseURL != "" {
		link.PublicURL = strings.TrimRight(baseURL, "/") + "/s/" + token
	} else {
		link.PublicURL = "/s/" + token
	}

	m.mu.Lock()
	m.shares[token] = link
	m.saveLocked()
	m.mu.Unlock()

	return link, err
}

func (m *Manager) DeleteShare(id string) bool {
	m.mu.Lock()
	_, ok := m.shares[id]
	if ok {
		delete(m.shares, id)
		m.saveLocked()
	}
	rem := len(m.shares)
	m.mu.Unlock()

	if rem == 0 {
		m.StopTunnel()
	}
	return ok
}

func (m *Manager) ListShares() []*ShareLink {
	m.PruneExpired()
	m.tunnelMu.Lock()
	activeBase := m.tunnelURL
	m.tunnelMu.Unlock()

	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.settings.Provider == "custom" && m.settings.PublicBaseURL != "" {
		activeBase = m.settings.PublicBaseURL
	}

	list := make([]*ShareLink, 0, len(m.shares))
	for _, s := range m.shares {
		cp := *s
		cp.PasswordHash = ""
		if activeBase != "" {
			cp.PublicURL = strings.TrimRight(activeBase, "/") + "/s/" + cp.ID
		}
		list = append(list, &cp)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list
}

func (m *Manager) GetShare(id string) (*ShareLink, bool) {
	m.PruneExpired()
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.shares[id]
	if !ok {
		return nil, false
	}
	cp := *s
	return &cp, true
}

func (m *Manager) IncrementDownload(id string) {
	m.mu.Lock()
	if s, ok := m.shares[id]; ok {
		s.Downloads++
		m.saveLocked()
	}
	m.mu.Unlock()
}

func (m *Manager) Status() (TunnelStatus, TunnelSettings) {
	m.mu.RLock()
	settings := m.settings
	settings.HasZrokToken = settings.ZrokToken != ""
	settings.ZrokToken = ""
	m.mu.RUnlock()

	m.tunnelMu.Lock()
	defer m.tunnelMu.Unlock()

	st := TunnelStatus{
		Running:       m.tunnelCmd != nil && m.tunnelURL != "",
		Provider:      settings.Provider,
		PublicBaseURL: m.tunnelURL,
		SharePort:     m.cfg.SharePort,
		Error:         m.tunnelErr,
	}
	if settings.Provider == "custom" {
		st.PublicBaseURL = settings.PublicBaseURL
		st.Running = settings.PublicBaseURL != ""
	}
	return st, settings
}

func (m *Manager) StopTunnel() {
	m.tunnelMu.Lock()
	defer m.tunnelMu.Unlock()
	if m.tunnelCancel != nil {
		m.tunnelCancel()
		m.tunnelCancel = nil
	}
	if m.tunnelCmd != nil && m.tunnelCmd.Process != nil {
		_ = m.tunnelCmd.Process.Kill()
	}
	m.tunnelCmd = nil
	m.tunnelURL = ""
	m.tunnelErr = ""
}

func (m *Manager) EnsureTunnel() (string, error) {
	m.mu.RLock()
	prov := m.settings.Provider
	customURL := m.settings.PublicBaseURL
	zrokToken := m.settings.ZrokToken
	m.mu.RUnlock()

	if prov == "custom" {
		if customURL == "" {
			return "", fmt.Errorf("custom public URL is not configured (set AWDEX_SHARE_PUBLIC_URL or configure in Share Manager)")
		}
		return customURL, nil
	}

	m.tunnelMu.Lock()
	defer m.tunnelMu.Unlock()

	if m.tunnelCmd != nil && m.tunnelURL != "" && m.tunnelProv == prov {
		// Check if process is still alive
		if m.tunnelCmd.ProcessState == nil {
			return m.tunnelURL, nil
		}
	}

	// Clean up any previous process
	if m.tunnelCancel != nil {
		m.tunnelCancel()
	}
	m.tunnelURL = ""
	m.tunnelErr = ""
	m.tunnelProv = prov

	switch prov {
	case "zrok":
		url, err := m.startZrokLocked(zrokToken)
		if err != nil {
			m.tunnelErr = err.Error()
		}
		return url, err
	default: // "cloudflared"
		url, err := m.startCloudflaredLocked()
		if err != nil {
			m.tunnelErr = err.Error()
		}
		return url, err
	}
}

func (m *Manager) ensureCloudflaredBinary() (string, error) {
	if p, err := exec.LookPath("cloudflared"); err == nil {
		return p, nil
	}
	binPath := filepath.Join(m.cfg.DataDir, "bin", "cloudflared")
	if fi, err := os.Stat(binPath); err == nil && fi.Size() > 1000000 {
		_ = os.Chmod(binPath, 0755)
		return binPath, nil
	}

	arch := "amd64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}
	dlURL := fmt.Sprintf("https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-%s", arch)
	log.Printf("[share] downloading cloudflared binary from %s ...", dlURL)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(dlURL)
	if err != nil {
		return "", fmt.Errorf("failed to download cloudflared: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cloudflared download returned HTTP %d", resp.StatusCode)
	}

	tmpPath := binPath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return "", err
	}
	f.Close()
	if err := os.Rename(tmpPath, binPath); err != nil {
		return "", err
	}
	log.Printf("[share] cloudflared installed at %s", binPath)
	return binPath, nil
}

func (m *Manager) startCloudflaredLocked() (string, error) {
	binPath, err := m.ensureCloudflaredBinary()
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithCancel(context.Background())
	target := fmt.Sprintf("http://127.0.0.1:%d", m.cfg.SharePort)
	cmd := exec.CommandContext(ctx, binPath, "tunnel", "--no-autoupdate", "--url", target)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return "", err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return "", fmt.Errorf("failed to start cloudflared: %w", err)
	}

	urlCh := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if match := cfURLRegex.FindString(line); match != "" {
				select {
				case urlCh <- match:
				default:
				}
			}
		}
	}()

	select {
	case pubURL := <-urlCh:
		m.tunnelCmd = cmd
		m.tunnelCancel = cancel
		m.tunnelURL = pubURL
		log.Printf("[share] cloudflared quick tunnel active: %s -> %s", pubURL, target)
		return pubURL, nil
	case <-time.After(18 * time.Second):
		cancel()
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("timed out waiting for cloudflared quick tunnel URL")
	}
}

func (m *Manager) startZrokLocked(token string) (string, error) {
	if token == "" {
		return "", fmt.Errorf("zrok token is required (set AWDEX_ZROK_TOKEN or enter token in Share Manager)")
	}
	binPath, err := exec.LookPath("zrok")
	if err != nil {
		binPath = filepath.Join(m.cfg.DataDir, "bin", "zrok")
		if _, err := os.Stat(binPath); err != nil {
			return "", fmt.Errorf("zrok binary not found in PATH or %s", binPath)
		}
	}

	zrokHome := filepath.Join(m.cfg.DataDir, ".zrok")
	_ = os.MkdirAll(zrokHome, 0700)

	// Enable environment if not yet enabled
	envFile := filepath.Join(zrokHome, "environment.json")
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		enCmd := exec.Command(binPath, "enable", token, "--headless")
		enCmd.Env = append(os.Environ(), "HOME="+m.cfg.DataDir, "ZROK_ROOT="+zrokHome)
		if out, err := enCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("zrok enable failed: %v (%s)", err, strings.TrimSpace(string(out)))
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	target := fmt.Sprintf("http://127.0.0.1:%d", m.cfg.SharePort)
	cmd := exec.CommandContext(ctx, binPath, "share", "public", "--headless", "--backend-mode", "proxy", target)
	cmd.Env = append(os.Environ(), "HOME="+m.cfg.DataDir, "ZROK_ROOT="+zrokHome)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		cancel()
		return "", fmt.Errorf("failed to start zrok: %w", err)
	}

	urlCh := make(chan string, 1)
	scanPipe := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			if match := zrokURLRegex.FindString(line); match != "" {
				select {
				case urlCh <- match:
				default:
				}
			}
		}
	}
	go scanPipe(stdout)
	go scanPipe(stderr)

	select {
	case pubURL := <-urlCh:
		m.tunnelCmd = cmd
		m.tunnelCancel = cancel
		m.tunnelURL = pubURL
		log.Printf("[share] zrok tunnel active: %s -> %s", pubURL, target)
		return pubURL, nil
	case <-time.After(18 * time.Second):
		cancel()
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("timed out waiting for zrok public share URL")
	}
}
