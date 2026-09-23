//go:build windows

package systheme

import "golang.org/x/sys/windows/registry"

// detect reads the "apps use light theme" personalisation flag.
func detect() (theme, source string) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return "", ""
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return "", ""
	}
	if v == 0 {
		return "dark", "registry"
	}
	return "light", "registry"
}
