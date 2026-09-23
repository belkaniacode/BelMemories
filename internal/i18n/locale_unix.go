//go:build !windows

package i18n

import (
	"os"
	"strings"
)

// systemLocale follows the POSIX lookup order for message language:
// LANGUAGE (first entry), LC_ALL, LC_MESSAGES, LANG.
func systemLocale() string {
	if v := os.Getenv("LANGUAGE"); v != "" {
		return strings.Split(v, ":")[0]
	}
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" && v != "C" && v != "POSIX" {
			return v
		}
	}
	return ""
}
