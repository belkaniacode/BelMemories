package fsutil

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Windows reserved device names (case-insensitive, with or without extension).
var reservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// maxNameBytes keeps names well below the 255-byte limit of most filesystems,
// leaving room for the "_N" suffix.
const maxNameBytes = 200

// SanitizeName makes a file name valid on Windows and Linux filesystems
// (NTFS, exFAT, ext4) so an archive can be moved between systems.
func SanitizeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r < 0x20, strings.ContainsRune(`<>:"/\|?*`, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimRight(b.String(), ". ")
	out = strings.TrimLeft(out, " ")
	if out == "" {
		out = "file"
	}

	ext := filepath.Ext(out)
	base := strings.TrimSuffix(out, ext)
	if reservedNames[strings.ToUpper(base)] {
		base = "_" + base
	}
	if len(base)+len(ext) > maxNameBytes {
		limit := maxNameBytes - len(ext)
		if limit < 1 {
			limit, ext = maxNameBytes, ""
		}
		base = truncateUTF8(base, limit)
	}
	if base == "" {
		base = "file"
	}
	return base + ext
}

func truncateUTF8(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	s = s[:limit]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
