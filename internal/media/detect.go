// Package media decides whether a file is a photo, a video or something else.
package media

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/h2non/filetype"

	"belmemories/internal/logging"
)

// Kind is the media kind of a file.
type Kind string

const (
	Photo Kind = "photo"
	Video Kind = "video"
	Other Kind = "other"
)

var photoExts = setOf(
	"jpg", "jpeg", "jpe", "jfif", "png", "heic", "heif", "webp", "tif", "tiff",
	"bmp", "gif", "avif", "svg", "ico",
	// RAW
	"cr2", "cr3", "crw", "nef", "nrw", "arw", "srf", "sr2", "dng", "orf", "rw2",
	"raf", "srw", "pef", "x3f", "3fr", "erf", "kdc", "mrw",
)

var rawExts = setOf(
	"cr2", "cr3", "crw", "nef", "nrw", "arw", "srf", "sr2", "dng", "orf", "rw2",
	"raf", "srw", "pef", "x3f", "3fr", "erf", "kdc", "mrw",
)

var videoExts = setOf(
	"mp4", "m4v", "mov", "3gp", "3g2", "avi", "mkv", "webm", "wmv", "flv", "mts",
	"m2ts", "ts", "mpg", "mpeg", "vob", "mod", "tod", "dv", "ogv",
)

// Sidecar / junk files that are never media even if the name suggests otherwise.
var ignoredExts = setOf("aae", "xmp", "thm", "lrv", "db", "ini", "ds_store")

// Extensions that are definitively something else, so magic sniffing is skipped.
// Keeps scanning fast on disks full of documents and program files.
var knownOtherExts = setOf(
	"txt", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "pdf", "zip", "rar", "7z",
	"gz", "tar", "exe", "dll", "so", "iso", "mp3", "wav", "flac", "ogg", "m4a",
	"aac", "wma", "html", "htm", "css", "js", "json", "xml", "csv", "log", "lnk",
	"sys", "msi", "apk", "jar", "class", "py", "go", "c", "h", "cpp", "java",
	"psd", "ai", "eps", "ttf", "otf", "woff", "woff2", "torrent", "bak", "tmp",
	"part", "crdownload", "sqlite", "dat", "bin", "cab", "epub", "fb2", "djvu",
	"rtf", "odt", "ods", "srt", "sub", "ass", "nfo", "md", "cue",
)

func setOf(items ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(items))
	for _, it := range items {
		m[it] = struct{}{}
	}
	return m
}

// Ext returns the lowercase extension without the dot.
func Ext(path string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
}

// IsRaw reports whether the extension is a camera RAW format.
func IsRaw(ext string) bool {
	_, ok := rawExts[ext]
	return ok
}

// IsIgnoredName reports names that should never be treated as media.
func IsIgnoredName(name string) bool {
	if strings.HasPrefix(name, "._") { // macOS resource forks
		return true
	}
	switch strings.ToLower(name) {
	case "thumbs.db", "desktop.ini", ".ds_store":
		return true
	}
	return false
}

// DetectByExt classifies by extension only. ok=false means the extension is
// unknown and content sniffing may help.
func DetectByExt(ext string) (kind Kind, ok bool) {
	if _, hit := photoExts[ext]; hit {
		return Photo, true
	}
	if _, hit := videoExts[ext]; hit {
		return Video, true
	}
	if _, hit := ignoredExts[ext]; hit {
		return Other, true
	}
	if _, hit := knownOtherExts[ext]; hit {
		return Other, true
	}
	return Other, false
}

// Detect classifies a file by extension, falling back to magic bytes for
// files without a known extension. size is used to skip empty files.
func Detect(path string, size int64) Kind {
	name := filepath.Base(path)
	if size <= 0 || IsIgnoredName(name) {
		return Other
	}
	ext := Ext(name)
	if kind, ok := DetectByExt(ext); ok {
		logging.Component("media").Debug("detected", "path", path, "kind", kind, "by", "ext")
		return kind
	}
	kind := sniff(path)
	logging.Component("media").Debug("detected", "path", path, "kind", kind, "by", "magic")
	return kind
}

func sniff(path string) Kind {
	f, err := os.Open(path)
	if err != nil {
		return Other
	}
	defer f.Close()
	head := make([]byte, 262)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		return Other
	}
	head = head[:n]
	switch {
	case filetype.IsImage(head):
		return Photo
	case filetype.IsVideo(head):
		return Video
	}
	return Other
}
