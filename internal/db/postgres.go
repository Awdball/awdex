package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/awdball/awdex/internal/model"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type postgresDB struct {
	db *sql.DB
}

func openPostgres(connStr string) (DB, error) {
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping postgres database: %w", err)
	}

	p := &postgresDB{db: db}
	if err := p.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return p, nil
}

func (p *postgresDB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS awdex_files (
		id BIGSERIAL PRIMARY KEY,
		path TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL,
		is_dir BOOLEAN NOT NULL,
		size BIGINT NOT NULL,
		mod_time BIGINT NOT NULL,
		mime_type TEXT,
		category TEXT NOT NULL,
		extension TEXT,
		has_thumb BOOLEAN DEFAULT FALSE,
		indexed_at BIGINT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_awdex_files_name ON awdex_files(name);
	CREATE INDEX IF NOT EXISTS idx_awdex_files_path ON awdex_files(path);
	CREATE INDEX IF NOT EXISTS idx_awdex_files_category ON awdex_files(category);
	`
	_, err := p.db.Exec(schema)
	return err
}

func (p *postgresDB) Close() error {
	return p.db.Close()
}

func (p *postgresDB) Type() string {
	return "postgres"
}

func (p *postgresDB) UpsertBatch(ctx context.Context, items []*model.FileItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO awdex_files (path, name, is_dir, size, mod_time, mime_type, category, extension, has_thumb, indexed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT(path) DO UPDATE SET
			name = EXCLUDED.name,
			is_dir = EXCLUDED.is_dir,
			size = EXCLUDED.size,
			mod_time = EXCLUDED.mod_time,
			mime_type = EXCLUDED.mime_type,
			category = EXCLUDED.category,
			extension = EXCLUDED.extension,
			has_thumb = EXCLUDED.has_thumb,
			indexed_at = EXCLUDED.indexed_at
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, it := range items {
		_, err := stmt.ExecContext(ctx,
			it.Path,
			it.Name,
			it.IsDir,
			it.Size,
			it.ModTime.Unix(),
			it.MimeType,
			it.Category,
			it.Extension,
			it.HasThumb,
			now,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (p *postgresDB) DeletePath(ctx context.Context, relPath string) error {
	_, err := p.db.ExecContext(ctx, "DELETE FROM awdex_files WHERE path = $1", relPath)
	return err
}

func (p *postgresDB) DeleteDirPrefix(ctx context.Context, dirPath string) error {
	prefix := strings.TrimSuffix(dirPath, "/") + "/%"
	_, err := p.db.ExecContext(ctx, "DELETE FROM awdex_files WHERE path = $1 OR path LIKE $2", dirPath, prefix)
	return err
}

func (p *postgresDB) Search(ctx context.Context, req model.SearchRequest) ([]*model.FileItem, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if req.Query != "" {
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR path ILIKE $%d)", argIdx, argIdx+1))
		pattern := "%" + req.Query + "%"
		args = append(args, pattern, pattern)
		argIdx += 2
	}

	if req.Category != "" && req.Category != "all" {
		where = append(where, fmt.Sprintf("category = $%d", argIdx))
		args = append(args, req.Category)
		argIdx++
	}

	if req.Path != "" && req.Path != "/" {
		prefix := strings.TrimSuffix(req.Path, "/") + "/%"
		where = append(where, fmt.Sprintf("path LIKE $%d", argIdx))
		args = append(args, prefix)
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM awdex_files WHERE %s", whereClause)
	var total int64
	if err := p.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT path, name, is_dir, size, mod_time, mime_type, category, extension, has_thumb
		FROM awdex_files
		WHERE %s
		ORDER BY is_dir DESC, name ASC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	selectArgs := append(args, limit, offset)
	rows, err := p.db.QueryContext(ctx, query, selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []*model.FileItem
	for rows.Next() {
		var it model.FileItem
		var modUnix int64

		if err := rows.Scan(
			&it.Path, &it.Name, &it.IsDir, &it.Size, &modUnix,
			&it.MimeType, &it.Category, &it.Extension, &it.HasThumb,
		); err != nil {
			return nil, 0, err
		}

		it.ModTime = time.Unix(modUnix, 0)
		items = append(items, &it)
	}

	return items, total, nil
}

func (p *postgresDB) GetStats(ctx context.Context) (int64, int64, int64, error) {
	var totalFiles, totalDirs, totalBytes int64
	row := p.db.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN is_dir = FALSE THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN is_dir = TRUE THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN is_dir = FALSE THEN size ELSE 0 END), 0)
		FROM awdex_files
	`)
	err := row.Scan(&totalFiles, &totalDirs, &totalBytes)
	return totalFiles, totalDirs, totalBytes, err
}
