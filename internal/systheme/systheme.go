// Package systheme detects whether the OS uses a light or dark colour scheme.
package systheme

import "belmemories/internal/logging"

// Detect returns "dark" or "light" for the current OS colour scheme.
// When the scheme cannot be determined it returns "light".
func Detect() string {
	t, src := detect()
	if t == "" {
		t, src = "light", "fallback"
	}
	logging.Component("systheme").Info("[FIX] system theme detected", "theme", t, "source", src)
	return t
}
