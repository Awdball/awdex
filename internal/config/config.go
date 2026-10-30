package config

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	RootDir          string        `json:"root_dir"`
	DataDir          string        `json:"data_dir"`
	Host             string        `json:"host"`
	Port             int           `json:"port"`
	DatabaseURL      string        `json:"database_url"`
	ReadOnly         bool          `json:"read_only"`
	EnableThumbnails bool          `json:"enable_thumbnails"`
	EnableVideoScrub bool          `json:"enable_video_scrub"`
	Title            string        `json:"title"`
	IndexInterval    time.Duration `json:"index_interval"`
	BaseURL          string        `json:"base_url"`
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		RootDir:          getEnv("AWDEX_ROOT_DIR", "/content"),
		DataDir:          getEnv("AWDEX_DATA_DIR", "/data"),
		Host:             getEnv("AWDEX_HOST", "0.0.0.0"),
		Port:             getEnvInt("AWDEX_PORT", 8080),
		DatabaseURL:      getEnv("AWDEX_DATABASE_URL", ""),
		ReadOnly:         getEnvBool("AWDEX_READ_ONLY", false),
		EnableThumbnails: getEnvBool("AWDEX_ENABLE_THUMBNAILS", true),
		EnableVideoScrub: getEnvBool("AWDEX_ENABLE_VIDEO_SCRUB", true),
		Title:            getEnv("AWDEX_TITLE", "awdex"),
		IndexInterval:    getEnvDuration("AWDEX_INDEX_INTERVAL", 10*time.Minute),
		BaseURL:          getEnv("AWDEX_BASE_URL", "/"),
	}

	// Flags override environment variables
	flagRootDir := flag.String("root", "", "Directory to serve/index (default: /content or current directory)")
	flagDataDir := flag.String("data", "", "Directory to store generated assets, thumbnails, and sqlite db (default: /data or ./data)")
	flagHost := flag.String("host", "", "Host to listen on")
	flagPort := flag.Int("port", 0, "Port to listen on")
	flagDbURL := flag.String("db", "", "Database URL (postgres://... or empty for embedded sqlite)")
	flagReadOnly := flag.Bool("read-only", false, "Disable uploads, modifications, and directory creation")
	flagTitle := flag.String("title", "", "Site title")
	flag.Parse()

	if *flagRootDir != "" {
		cfg.RootDir = *flagRootDir
	}
	if *flagDataDir != "" {
		cfg.DataDir = *flagDataDir
	}
	if *flagHost != "" {
		cfg.Host = *flagHost
	}
	if *flagPort != 0 {
		cfg.Port = *flagPort
	}
	if *flagDbURL != "" {
		cfg.DatabaseURL = *flagDbURL
	}
	if *flagReadOnly {
		cfg.ReadOnly = true
	}
	if *flagTitle != "" {
		cfg.Title = *flagTitle
	}

	// Fallback to relative local dirs if default container paths don't exist
	if !dirExists(cfg.RootDir) {
		cwd, err := os.Getwd()
		if err == nil {
			cfg.RootDir = cwd
		}
	}
	if !dirExists(cfg.DataDir) {
		cfg.DataDir = filepath.Join(cfg.RootDir, ".awdex-data")
	}

	// Ensure DataDir exists
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "thumbnails"), 0755); err != nil {
		return nil, err
	}

	return cfg, nil
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return strings.TrimSpace(val)
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			return n
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		val = strings.ToLower(strings.TrimSpace(val))
		return val == "1" || val == "true" || val == "yes" || val == "on"
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(strings.TrimSpace(val)); err == nil {
			return d
		}
	}
	return defaultVal
}
