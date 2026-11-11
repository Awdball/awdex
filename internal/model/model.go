package model

import (
	"time"
)

// FileItem represents a file or directory for browsing and searching
type FileItem struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	IsDir     bool      `json:"is_dir"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"mod_time"`
	MimeType  string    `json:"mime_type,omitempty"`
	Category  string    `json:"category"`
	Extension string    `json:"extension,omitempty"`
	HasThumb  bool      `json:"has_thumb"`
	ThumbURL  string    `json:"thumb_url,omitempty"`
	ScrubURL  string    `json:"scrub_url,omitempty"`
	ItemCount int       `json:"item_count,omitempty"`
}

// TreeNode represents a node in the expandable sidebar tree
type TreeNode struct {
	Name        string      `json:"name"`
	Path        string      `json:"path"`
	HasChildren bool        `json:"has_children"`
	SubCount    int         `json:"sub_count"`
	Children    []*TreeNode `json:"children,omitempty"`
}

// BrowseResponse returns current directory listings and breadcrumbs
type BrowseResponse struct {
	Path        string      `json:"path"`
	ParentPath  string      `json:"parent_path"`
	Breadcrumbs []Crumb     `json:"breadcrumbs"`
	Items       []*FileItem `json:"items"`
	TotalCount  int         `json:"total_count"`
	TotalDirs   int         `json:"total_dirs"`
	TotalFiles  int         `json:"total_files"`
	TotalBytes  int64       `json:"total_bytes"`
	ReadOnly    bool        `json:"read_only"`
}

// Crumb represents one level in the breadcrumb trail
type Crumb struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// SearchRequest parameters
type SearchRequest struct {
	Query    string `json:"query"`
	Path     string `json:"path"`
	Category string `json:"category"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
}

// SearchResponse results
type SearchResponse struct {
	Items      []*FileItem `json:"items"`
	TotalCount int64       `json:"total_count"`
	Query      string      `json:"query"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
}

// IndexerStatus represents the explicit state machine snapshot of the background indexer
type IndexerStatus struct {
	State          string     `json:"state"`
	Running        bool       `json:"running"`
	Generation     uint64     `json:"generation"`
	ScannedCount   int64      `json:"scanned_count"`
	LastIndexedAt  *time.Time `json:"last_indexed_at,omitempty"`
	LastDurationMs int64      `json:"last_duration_ms,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
}

// ServerInfo returns system and config statistics
type ServerInfo struct {
	Title          string         `json:"title"`
	Version        string         `json:"version"`
	ReadOnly       bool           `json:"read_only"`
	DatabaseType   string         `json:"database_type"`
	IndexedFiles   int64          `json:"indexed_files"`
	IndexedDirs    int64          `json:"indexed_dirs"`
	IndexedBytes   int64          `json:"indexed_bytes"`
	IndexState     string         `json:"index_state"`
	IndexStatus    *IndexerStatus `json:"index_status,omitempty"`
	FFmpegPresent  bool           `json:"ffmpeg_present"`
	PopplerPresent bool           `json:"poppler_present"`
}

// ArchiveEntry represents a file inside an inspected ZIP/tar file
type ArchiveEntry struct {
	Name    string    `json:"name"`
	IsDir   bool      `json:"is_dir"`
	Size    uint64    `json:"size"`
	ModTime time.Time `json:"mod_time"`
}
