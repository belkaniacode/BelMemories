//go:build linux

package systheme

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// detect checks, in order: the freedesktop color-scheme key (GNOME, KDE and
// most modern desktops mirror it), KDE's kdeglobals colour scheme, and finally
// the GTK theme name. WebKitGTK's prefers-color-scheme is not reliable here: it
// follows the GTK theme, which often stays "Breeze"/"Adwaita" on a dark desktop.
func detect() (theme, source string) {
	if v := gsettings("color-scheme"); v != "" {
		switch {
		case strings.Contains(v, "dark"):
			return "dark", "gsettings color-scheme"
		case strings.Contains(v, "light"):
			return "light", "gsettings color-scheme"
		}
	}
	if v := kdeColorScheme(); v != "" {
		return darkIf(strings.Contains(strings.ToLower(v), "dark")), "kdeglobals"
	}
	if v := os.Getenv("GTK_THEME"); v != "" {
		return darkIf(strings.Contains(strings.ToLower(v), "dark")), "GTK_THEME"
	}
	if v := gsettings("gtk-theme"); v != "" {
		return darkIf(strings.Contains(v, "dark")), "gsettings gtk-theme"
	}
	return "", ""
}

func darkIf(dark bool) string {
	if dark {
		return "dark"
	}
	return "light"
}

// gsettings reads an org.gnome.desktop.interface key, lower-cased and unquoted.
func gsettings(key string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gsettings", "get", "org.gnome.desktop.interface", key).Output()
	if err != nil {
		return ""
	}
	return strings.Trim(strings.ToLower(strings.TrimSpace(string(out))), "'")
}

// kdeColorScheme returns [General] ColorScheme from ~/.config/kdeglobals.
func kdeColorScheme() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	f, err := os.Open(filepath.Join(dir, "kdeglobals"))
	if err != nil {
		return ""
	}
	defer f.Close()
	inGeneral := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			inGeneral = line == "[General]"
			continue
		}
		if k, v, ok := strings.Cut(line, "="); inGeneral && ok && strings.TrimSpace(k) == "ColorScheme" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
