package db

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/awdball/awdex/internal/model"
	_ "modernc.org/sqlite"
)

type sqliteDB struct {
	db *sql.DB
}

func openSQLite(dataDir string) (DB, error) {
	dbPath := filepath.Join(dataDir, "awdex.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	s := &sqliteDB{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *sqliteDB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL,
		is_dir INTEGER NOT NULL,
		size INTEGER NOT NULL,
		mod_time INTEGER NOT NULL,
		mime_type TEXT,
		category TEXT NOT NULL,
		extension TEXT,
		has_thumb INTEGER DEFAULT 0,
		indexed_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_files_name ON files(name COLLATE NOCASE);
	CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
	CREATE INDEX IF NOT EXISTS idx_files_category ON files(category);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *sqliteDB) Close() error {
	return s.db.Close()
}

func (s *sqliteDB) Type() string {
	return "sqlite"
}

func (s *sqliteDB) UpsertBatch(ctx context.Context, items []*model.FileItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO files (path, name, is_dir, size, mod_time, mime_type, category, extension, has_thumb, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			name=excluded.name,
			is_dir=excluded.is_dir,
			size=excluded.size,
			mod_time=excluded.mod_time,
			mime_type=excluded.mime_type,
			category=excluded.category,
			extension=excluded.extension,
			has_thumb=excluded.has_thumb,
			indexed_at=excluded.indexed_at
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, it := range items {
		isDir := 0
		if it.IsDir {
			isDir = 1
		}
		hasThumb := 0
		if it.HasThumb {
			hasThumb = 1
		}

		_, err := stmt.ExecContext(ctx,
			it.Path,
			it.Name,
			isDir,
			it.Size,
			it.ModTime.Unix(),
			it.MimeType,
			it.Category,
			it.Extension,
			hasThumb,
			now,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *sqliteDB) DeletePath(ctx context.Context, relPath string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM files WHERE path = ?", relPath)
	return err
}

func (s *sqliteDB) DeleteDirPrefix(ctx context.Context, dirPath string) error {
	prefix := strings.TrimSuffix(dirPath, "/") + "/%"
	_, err := s.db.ExecContext(ctx, "DELETE FROM files WHERE path = ? OR path LIKE ?", dirPath, prefix)
	return err
}

func (s *sqliteDB) Search(ctx context.Context, req model.SearchRequest) ([]*model.FileItem, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}

	if req.Query != "" {
		where = append(where, "(name LIKE ? OR path LIKE ?)")
		pattern := "%" + req.Query + "%"
		args = append(args, pattern, pattern)
	}

	if req.Category != "" && req.Category != "all" {
		where = append(where, "category = ?")
		args = append(args, req.Category)
	}

	if req.Path != "" && req.Path != "/" {
		prefix := strings.TrimSuffix(req.Path, "/") + "/%"
		where = append(where, "path LIKE ?")
		args = append(args, prefix)
	}

	whereClause := strings.Join(where, " AND ")

	// Count query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM files WHERE %s", whereClause)
	var total int64
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
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

	// Result query
	query := fmt.Sprintf(`
		SELECT path, name, is_dir, size, mod_time, mime_type, category, extension, has_thumb
		FROM files
		WHERE %s
		ORDER BY is_dir DESC, name ASC
		LIMIT ? OFFSET ?
	`, whereClause)

	selectArgs := append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []*model.FileItem
	for rows.Next() {
		var it model.FileItem
		var isDirInt, hasThumbInt int
		var modUnix int64

		if err := rows.Scan(
			&it.Path, &it.Name, &isDirInt, &it.Size, &modUnix,
			&it.MimeType, &it.Category, &it.Extension, &hasThumbInt,
		); err != nil {
			return nil, 0, err
		}

		it.IsDir = isDirInt == 1
		it.HasThumb = hasThumbInt == 1
		it.ModTime = time.Unix(modUnix, 0)
		items = append(items, &it)
	}

	return items, total, nil
}

func (s *sqliteDB) GetStats(ctx context.Context) (int64, int64, int64, error) {
	var totalFiles, totalDirs, totalBytes int64
	row := s.db.QueryRowContext(ctx, `
		SELECT 
			COALESCE(SUM(CASE WHEN is_dir = 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN is_dir = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN is_dir = 0 THEN size ELSE 0 END), 0)
		FROM files
	`)
	err := row.Scan(&totalFiles, &totalDirs, &totalBytes)
	return totalFiles, totalDirs, totalBytes, err
}
