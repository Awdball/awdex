package db

import (
	"context"
	"strings"

	"github.com/awdball/awdex/internal/config"
	"github.com/awdball/awdex/internal/model"
)

// DB defines the storage interface for fast indexed search and metadata
type DB interface {
	Close() error
	UpsertBatch(ctx context.Context, items []*model.FileItem) error
	DeletePath(ctx context.Context, relPath string) error
	DeleteDirPrefix(ctx context.Context, dirPath string) error
	Search(ctx context.Context, req model.SearchRequest) ([]*model.FileItem, int64, error)
	GetStats(ctx context.Context) (int64, int64, int64, error)
	Type() string
}

// Open creates either an embedded SQLite database or a PostgreSQL connection
func Open(cfg *config.Config) (DB, error) {
	if cfg.DatabaseURL != "" && strings.HasPrefix(cfg.DatabaseURL, "postgres") {
		return openPostgres(cfg.DatabaseURL)
	}
	return openSQLite(cfg.DataDir)
}
