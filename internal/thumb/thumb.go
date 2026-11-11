package thumb

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

// ErrNoCoverArt indicates an audio file or directory has no embedded/available cover art.
var ErrNoCoverArt = errors.New("no cover art available")

// jobState represents the explicit state of an in-flight thumbnail/scrub generation job.
type jobState string

const (
	jobStateRunning   jobState = "running"
	jobStateSucceeded jobState = "succeeded"
	jobStateNoCover   jobState = "no_cover"
	jobStateFailed    jobState = "failed"
)

type inflightJob struct {
	done  chan struct{}
	state jobState
	path  string
	err   error
}

type Manager struct {
	cfg        *config.Config
	thumbDir   string
	hasFFmpeg  bool
	hasPoppler bool
	inFlight   sync.Map // cacheKey -> *inflightJob
	noCover    sync.Map // cacheKey -> bool (negative cache for audio/dirs without cover art)
	sem        chan struct{}
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

// runSingleFlight executes fn once per cacheKey across concurrent callers using an explicit job state machine.
func (m *Manager) runSingleFlight(cacheKey, cachePath string, fn func(tmpPath string) error) (string, error) {
	if _, ok := m.noCover.Load(cacheKey); ok {
		return "", ErrNoCoverArt
	}
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	newJob := &inflightJob{
		done:  make(chan struct{}),
		state: jobStateRunning,
	}
	actual, loaded := m.inFlight.LoadOrStore(cacheKey, newJob)
	job := actual.(*inflightJob)
	if loaded {
		<-job.done
		return job.path, job.err
	}

	defer func() {
		close(job.done)
		m.inFlight.Delete(cacheKey)
	}()

	// Re-check disk cache after acquiring single-flight ownership
	if _, err := os.Stat(cachePath); err == nil {
		job.state = jobStateSucceeded
		job.path = cachePath
		return cachePath, nil
	}

	m.sem <- struct{}{}
	defer func() { <-m.sem }()

	tmpPath := cachePath + ".tmp" + filepath.Ext(cachePath)
	err := fn(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		if errors.Is(err, ErrNoCoverArt) {
			job.state = jobStateNoCover
			m.noCover.Store(cacheKey, true)
		} else {
			job.state = jobStateFailed
		}
		job.err = err
		return "", err
	}

	if err := os.Rename(tmpPath, cachePath); err != nil {
		_ = os.Remove(tmpPath)
		job.state = jobStateFailed
		job.err = err
		return "", err
	}

	job.state = jobStateSucceeded
	job.path = cachePath
	return cachePath, nil
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

	return m.runSingleFlight(cacheKey, cachePath, func(tmpPath string) error {
		category, _, ext := util.DetectCategory(fi.Name(), fi.IsDir())
		switch category {
		case util.CatImage:
			return m.generateImageThumb(fullPath, tmpPath)
		case util.CatVideo:
			return m.generateVideoThumb(fullPath, tmpPath)
		case util.CatAudio:
			return m.generateAudioThumb(fullPath, tmpPath)
		case util.CatDocument:
			if ext == ".pdf" && m.hasPoppler {
				return m.generatePdfThumb(fullPath, tmpPath)
			}
			return fmt.Errorf("unsupported document thumbnail")
		default:
			return fmt.Errorf("unsupported category for thumbnail: %s", category)
		}
	})
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

	return m.runSingleFlight(cacheKey, cachePath, func(tmpPath string) error {
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
			return fmt.Errorf("ffmpeg scrub error: %v, output: %s", err, string(out))
		}
		return nil
	})
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
		return fmt.Errorf("%w: %v", ErrNoCoverArt, err)
	}

	pic := metadata.Picture()
	if pic == nil || len(pic.Data) == 0 {
		return ErrNoCoverArt
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

func (m *Manager) findDirectoryCover(fullPath string, maxDepth int) string {
	if maxDepth < 0 {
		return ""
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return ""
	}

	// 1. Look for explicit cover files first
	coverNames := []string{"cover.jpg", "cover.png", "folder.jpg", "folder.png", "poster.jpg", "poster.png", "thumb.jpg", "thumb.png"}
	for _, cn := range coverNames {
		for _, e := range entries {
			if strings.EqualFold(e.Name(), cn) {
				return filepath.Join(fullPath, e.Name())
			}
		}
	}

	// 2. Fall back to first thumbnailable image or video
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		cat, _, ext := util.DetectCategory(e.Name(), false)
		if m.CanThumbnail(cat, ext) {
			return filepath.Join(fullPath, e.Name())
		}
	}

	// 3. Recurse into subdirectories if depth allows
	if maxDepth > 0 {
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == "awdex" {
				continue
			}
			subCover := m.findDirectoryCover(filepath.Join(fullPath, e.Name()), maxDepth-1)
			if subCover != "" {
				return subCover
			}
		}
	}

	return ""
}

func (m *Manager) getDirectoryThumbnail(relPath, fullPath string, fi os.FileInfo) (string, error) {
	cacheKey := m.hashKey(relPath, fi.ModTime(), fi.Size(), "thumb-dir")
	cachePath := filepath.Join(m.thumbDir, cacheKey+".jpg")

	return m.runSingleFlight(cacheKey, cachePath, func(tmpPath string) error {
		targetFile := m.findDirectoryCover(fullPath, 2)
		if targetFile == "" {
			return ErrNoCoverArt
		}

		tFi, err := os.Stat(targetFile)
		if err != nil {
			return err
		}

		cat, _, ext := util.DetectCategory(tFi.Name(), false)
		switch cat {
		case util.CatImage:
			return m.generateImageThumb(targetFile, tmpPath)
		case util.CatVideo:
			return m.generateVideoThumb(targetFile, tmpPath)
		case util.CatAudio:
			return m.generateAudioThumb(targetFile, tmpPath)
		case util.CatDocument:
			if ext == ".pdf" && m.hasPoppler {
				return m.generatePdfThumb(targetFile, tmpPath)
			}
			return fmt.Errorf("unsupported doc")
		default:
			return fmt.Errorf("unsupported thumb target: %s", targetFile)
		}
	})
}
