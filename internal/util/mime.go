package util

import (
	"fmt"
	"mime"
	"path"
	"path/filepath"
	"strings"
)

const (
	CatFolder   = "folder"
	CatImage    = "image"
	CatVideo    = "video"
	CatAudio    = "audio"
	CatDocument = "document"
	CatText     = "text"
	CatCode     = "code"
	CatArchive  = "archive"
	CatBinary   = "binary"
)

var videoExtensions = map[string]string{
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".webm": "video/webm",
	".mkv":  "video/x-matroska",
	".flv":  "video/x-flv",
	".avi":  "video/x-msvideo",
	".wmv":  "video/x-ms-wmv",
	".asf":  "video/x-ms-asf",
	".mov":  "video/quicktime",
	".qt":   "video/quicktime",
	".ts":   "video/mp2t",
	".m2ts": "video/mp2t",
	".mts":  "video/mp2t",
	".vob":  "video/x-ms-vob",
	".ogv":  "video/ogg",
	".3gp":  "video/3gpp",
	".3g2":  "video/3gpp2",
	".rm":   "video/vnd.rn-realvideo",
	".rmvb": "video/vnd.rn-realvideo",
	".mpg":  "video/mpeg",
	".mpeg": "video/mpeg",
	".divx": "video/x-msvideo",
}

var audioExtensions = map[string]string{
	".mp3":  "audio/mpeg",
	".flac": "audio/flac",
	".wav":  "audio/wav",
	".ogg":  "audio/ogg",
	".oga":  "audio/ogg",
	".m4a":  "audio/mp4",
	".aac":  "audio/aac",
	".opus": "audio/opus",
	".wma":  "audio/x-ms-wma",
	".aiff": "audio/aiff",
	".aif":  "audio/aiff",
	".alac": "audio/alac",
	".mka":  "audio/x-matroska",
	".ape":  "audio/x-ape",
	".dsf":  "audio/dsf",
	".dff":  "audio/dff",
}

var imageExtensions = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".bmp":  "image/bmp",
	".tiff": "image/tiff",
	".tif":  "image/tiff",
	".ico":  "image/x-icon",
	".heic": "image/heic",
	".heif": "image/heif",
	".avif": "image/avif",
}

var codeExtensions = map[string]bool{
	".go": true, ".py": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true,
	".c": true, ".cpp": true, ".cc": true, ".h": true, ".hpp": true, ".rs": true,
	".java": true, ".kt": true, ".scala": true, ".php": true, ".rb": true, ".sh": true,
	".bash": true, ".zsh": true, ".ps1": true, ".bat": true, ".cmd": true, ".sql": true,
	".html": true, ".htm": true, ".css": true, ".scss": true, ".sass": true, ".less": true,
	".vue": true, ".svelte": true, ".json": true, ".xml": true, ".yaml": true, ".yml": true,
	".toml": true, ".ini": true, ".conf": true, ".dockerfile": true, ".makefile": true,
	".lua": true, ".swift": true, ".dart": true, ".r": true, ".diff": true, ".patch": true,
}

var docExtensions = map[string]bool{
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".odt": true, ".ods": true, ".odp": true,
	".rtf": true, ".epub": true, ".mobi": true, ".pages": true, ".numbers": true,
}

var archiveExtensions = map[string]bool{
	".zip": true, ".tar": true, ".gz": true, ".tgz": true, ".bz2": true,
	".tbz2": true, ".xz": true, ".txz": true, ".7z": true, ".rar": true,
	".iso": true, ".dmg": true,
}

// DetectCategory determines the primary category and MIME type for a file
func DetectCategory(filename string, isDir bool) (string, string, string) {
	if isDir {
		return CatFolder, "inode/directory", ""
	}

	ext := strings.ToLower(filepath.Ext(filename))
	base := strings.ToLower(filepath.Base(filename))

	if base == "dockerfile" || base == "makefile" || base == "gemfile" || base == "cmakelists.txt" || base == "caddyfile" {
		return CatCode, "text/plain", ext
	}

	// Check explicit video extensions first
	if mimeType, ok := videoExtensions[ext]; ok {
		return CatVideo, mimeType, ext
	}

	// Check explicit audio extensions
	if mimeType, ok := audioExtensions[ext]; ok {
		return CatAudio, mimeType, ext
	}

	// Check explicit image extensions
	if mimeType, ok := imageExtensions[ext]; ok {
		return CatImage, mimeType, ext
	}

	// Check system MIME
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		switch ext {
		case ".md", ".markdown":
			mimeType = "text/markdown"
		case ".7z":
			mimeType = "application/x-7z-compressed"
		case ".rar":
			mimeType = "application/x-rar-compressed"
		default:
			mimeType = "application/octet-stream"
		}
	}

	if strings.HasPrefix(mimeType, "image/") {
		return CatImage, mimeType, ext
	}
	if strings.HasPrefix(mimeType, "video/") {
		return CatVideo, mimeType, ext
	}
	if strings.HasPrefix(mimeType, "audio/") {
		return CatAudio, mimeType, ext
	}

	if docExtensions[ext] {
		return CatDocument, mimeType, ext
	}
	if archiveExtensions[ext] {
		return CatArchive, mimeType, ext
	}
	if codeExtensions[ext] {
		return CatCode, mimeType, ext
	}
	if ext == ".txt" || ext == ".log" || ext == ".md" || ext == ".csv" || strings.HasPrefix(mimeType, "text/") {
		return CatText, mimeType, ext
	}

	return CatBinary, mimeType, ext
}

// CleanRelativePath normalizes and prevents directory traversal attacks
func CleanRelativePath(p string) string {
	cleaned := path.Clean("/" + strings.TrimSpace(p))
	if cleaned == "." || cleaned == "/" {
		return "/"
	}
	return cleaned
}

// FormatBytes converts raw bytes to a human readable string
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
