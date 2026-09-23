package scanner

import (
	"runtime"
	"strings"
)

// pathEqual compares paths case-insensitively on Windows.
func pathEqual(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
