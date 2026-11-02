package thumb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/awdball/awdex/internal/config"
	"github.com/awdball/awdex/internal/util"
	"github.com/dhowden/tag"
	"github.com/disintegration/imaging"
)

type Manager struct {
	cfg            *config.Config
	thumbDir       string
	hasFFmpeg      bool
	hasPoppler     bool
	inFlight       sync.Map
	sem            chan struct{}
}

func NewManager(cfg *config.Config) *Manager {
	_, ffmpegErr := exec.LookPath("ffmpeg")
	_, popplerErr := exec.LookPath("pdftoppm")

	thumbDir := filepath.Join(cfg.DataDir, "thumbnails")
	_ = os.MkdirAll(thumbDir, 0755)

	return &Manager{
		cfg:        cfg,
		thumbDir:   thumbDir,
		hasFFmpeg:  ffmpegErr == nil,
		hasPoppler: popplerErr == nil,
		sem:        make(chan struct{}, 2), // max 2 concurrent ffmpeg/media extractions
	}
}

func (m *Manager) HasFFmpeg() bool {
	return m.hasFFmpeg
}

func (m *Manager) HasPoppler() bool {
	return m.hasPoppler
}

// CanThumbnail returns true if awdex can generate a thumbnail for this item
func (m *Manager) CanThumbnail(category, ext string) bool {
	if !m.cfg.EnableThumbnails {
		return false
	}
	switch category {
	case util.CatImage:
		return true
	case util.CatVideo:
		return m.hasFFmpeg
	case util.CatAudio:
		return true
	case util.CatDocument:
		return ext == ".pdf" && m.hasPoppler
	default:
		return false
	}
}

// GetThumbnail returns the path to the cached thumbnail, generating it if needed
func (m *Manager) GetThumbnail(relPath string) (string, error) {
	fullPath := filepath.Join(m.cfg.RootDir, filepath.FromSlash(strings.TrimPrefix(relPath, "/")))
	fi, err := os.Stat(fullPath)
	if err != nil {
		return "", err
	}

	if fi.IsDir() {
		return m.getDirectoryThumbnail(relPath, fullPath, fi)
	}

	cacheKey := m.hashKey(relPath, fi.ModTime(), fi.Size(), "thumb")
	cachePath := filepath.Join(m.thumbDir, cacheKey+".jpg")

	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	// In-flight deduplication
	val, _ := m.inFlight.LoadOrStore(cacheKey, &sync.Mutex{})
	mtx := val.(*sync.Mutex)
	mtx.Lock()
	defer mtx.Unlock()
	defer m.inFlight.Delete(cacheKey)

	// Check again in case another goroutine just finished it
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	m.sem <- struct{}{}
	defer func() { <-m.sem }()

	category, _, ext := util.DetectCategory(fi.Name(), fi.IsDir())
	tmpPath := cachePath + ".tmp.jpg"

	switch category {
	case util.CatImage:
		err = m.generateImageThumb(fullPath, tmpPath)
	case util.CatVideo:
		err = m.generateVideoThumb(fullPath, tmpPath)
	case util.CatAudio:
		err = m.generateAudioThumb(fullPath, tmpPath)
	case util.CatDocument:
		if ext == ".pdf" && m.hasPoppler {
			err = m.generatePdfThumb(fullPath, tmpPath)
		} else {
			err = fmt.Errorf("unsupported document thumbnail")
		}
	default:
		err = fmt.Errorf("unsupported category for thumbnail: %s", category)
	}

	if err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}

	if err := os.Rename(tmpPath, cachePath); err != nil {
		return "", err
	}

	return cachePath, nil
}

// GetVideoScrub returns the path to an animated webp preview for videos
func (m *Manager) GetVideoScrub(relPath string) (string, error) {
	if !m.hasFFmpeg || !m.cfg.EnableVideoScrub {
		return "", fmt.Errorf("video scrub disabled or ffmpeg not present")
	}

	fullPath := filepath.Join(m.cfg.RootDir, filepath.FromSlash(strings.TrimPrefix(relPath, "/")))
	fi, err := os.Stat(fullPath)
	if err != nil {
		return "", err
	}

	cacheKey := m.hashKey(relPath, fi.ModTime(), fi.Size(), "scrub")
	cachePath := filepath.Join(m.thumbDir, cacheKey+".webp")

	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	val, _ := m.inFlight.LoadOrStore(cacheKey, &sync.Mutex{})
	mtx := val.(*sync.Mutex)
	mtx.Lock()
	defer mtx.Unlock()
	defer m.inFlight.Delete(cacheKey)

	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	m.sem <- struct{}{}
	defer func() { <-m.sem }()

	tmpPath := cachePath + ".tmp.webp"
	// Extract 2.5 seconds clip starting at 2s, scale to 320px width, 8 fps
	cmd := exec.Command("ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-threads", "2",
		"-y",
		"-ss", "00:00:02",
		"-t", "2.5",
		"-i", fullPath,
		"-an",
		"-vf", "fps=8,scale=320:-1:flags=lanczos",
		"-loop", "0",
		"-quality", "65",
		tmpPath,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("ffmpeg scrub error: %v, output: %s", err, string(out))
	}

	if err := os.Rename(tmpPath, cachePath); err != nil {
		return "", err
	}

	return cachePath, nil
}

func (m *Manager) generateImageThumb(src, dst string) error {
	srcImg, err := imaging.Open(src, imaging.AutoOrientation(true))
	if err != nil {
		return err
	}

	thumb := imaging.Fit(srcImg, 480, 480, imaging.Lanczos)
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	return jpeg.Encode(out, thumb, &jpeg.Options{Quality: 82})
}

func (m *Manager) generateVideoThumb(src, dst string) error {
	if !m.hasFFmpeg {
		return fmt.Errorf("ffmpeg not available")
	}

	cmd := exec.Command("ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-threads", "1",
		"-y",
		"-ss", "00:00:02",
		"-i", src,
		"-an",
		"-vframes", "1",
		"-vf", "scale=480:-1",
		"-q:v", "3",
		"-f", "image2",
		"-vcodec", "mjpeg",
		dst,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		// Try at 00:00:00 in case the video is very short
		cmd2 := exec.Command("ffmpeg",
			"-hide_banner",
			"-loglevel", "error",
			"-threads", "1",
			"-y",
			"-ss", "00:00:00",
			"-i", src,
			"-an",
			"-vframes", "1",
			"-vf", "scale=480:-1",
			"-q:v", "3",
			"-f", "image2",
			"-vcodec", "mjpeg",
			dst,
		)
		if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
			return fmt.Errorf("ffmpeg video thumb failed: %v, out: %s / %s", err, string(out), string(out2))
		}
	}

	return nil
}

func (m *Manager) generateAudioThumb(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	metadata, err := tag.ReadFrom(f)
	if err != nil {
		return err
	}

	pic := metadata.Picture()
	if pic == nil || len(pic.Data) == 0 {
		return fmt.Errorf("no embedded album art")
	}

	img, _, err := image.Decode(strings.NewReader(string(pic.Data)))
	if err != nil {
		return err
	}

	thumb := imaging.Fit(img, 480, 480, imaging.Lanczos)
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	return jpeg.Encode(out, thumb, &jpeg.Options{Quality: 85})
}

func (m *Manager) generatePdfThumb(src, dst string) error {
	if !m.hasPoppler {
		return fmt.Errorf("pdftoppm not available")
	}

	tmpPrefix := dst + ".pdfpage"
	cmd := exec.Command("pdftoppm",
		"-jpeg",
		"-f", "1",
		"-l", "1",
		"-scale-to", "480",
		src,
		tmpPrefix,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pdftoppm failed: %v, out: %s", err, string(out))
	}

	// pdftoppm generates tmpPrefix-1.jpg
	genFile := tmpPrefix + "-1.jpg"
	if _, err := os.Stat(genFile); err != nil {
		genFile = tmpPrefix + "-01.jpg"
	}

	if err := os.Rename(genFile, dst); err != nil {
		return err
	}

	return nil
}

func (m *Manager) hashKey(relPath string, modTime time.Time, size int64, prefix string) string {
	raw := fmt.Sprintf("%s:%d:%d:%s", relPath, modTime.Unix(), size, prefix)
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:16])
}

func (m *Manager) getDirectoryThumbnail(relPath, fullPath string, fi os.FileInfo) (string, error) {
	cacheKey := m.hashKey(relPath, fi.ModTime(), fi.Size(), "thumb-dir")
	cachePath := filepath.Join(m.thumbDir, cacheKey+".jpg")

	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return "", err
	}

	// 1. Look for explicit cover files first
	coverNames := []string{"cover.jpg", "cover.png", "folder.jpg", "folder.png", "poster.jpg", "poster.png", "thumb.jpg", "thumb.png"}
	var targetFile string

	for _, cn := range coverNames {
		for _, e := range entries {
			if strings.EqualFold(e.Name(), cn) {
				targetFile = filepath.Join(fullPath, e.Name())
				break
			}
		}
		if targetFile != "" {
			break
		}
	}

	// 2. Fall back to first thumbnailable image or video
	if targetFile == "" {
		for _, e := range entries {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			cat, _, ext := util.DetectCategory(e.Name(), false)
			if m.CanThumbnail(cat, ext) {
				targetFile = filepath.Join(fullPath, e.Name())
				break
			}
		}
	}

	if targetFile == "" {
		return "", fmt.Errorf("no thumbnailable files in directory")
	}

	// In-flight deduplication
	val, _ := m.inFlight.LoadOrStore(cacheKey, &sync.Mutex{})
	mtx := val.(*sync.Mutex)
	mtx.Lock()
	defer mtx.Unlock()
	defer m.inFlight.Delete(cacheKey)

	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	m.sem <- struct{}{}
	defer func() { <-m.sem }()

	tmpPath := cachePath + ".tmp.jpg"
	tFi, err := os.Stat(targetFile)
	if err != nil {
		return "", err
	}

	cat, _, ext := util.DetectCategory(tFi.Name(), false)
	switch cat {
	case util.CatImage:
		err = m.generateImageThumb(targetFile, tmpPath)
	case util.CatVideo:
		err = m.generateVideoThumb(targetFile, tmpPath)
	case util.CatAudio:
		err = m.generateAudioThumb(targetFile, tmpPath)
	case util.CatDocument:
		if ext == ".pdf" && m.hasPoppler {
			err = m.generatePdfThumb(targetFile, tmpPath)
		} else {
			err = fmt.Errorf("unsupported doc")
		}
	default:
		err = fmt.Errorf("unsupported thumb target: %s", targetFile)
	}

	if err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}

	if err := os.Rename(tmpPath, cachePath); err != nil {
		return "", err
	}

	return cachePath, nil
}
