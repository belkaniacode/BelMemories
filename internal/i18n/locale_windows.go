//go:build windows

package i18n

import "golang.org/x/sys/windows"

// systemLocale returns the first preferred UI language of the user, e.g. "ru-RU".
func systemLocale() string {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil || len(langs) == 0 {
		return ""
	}
	return langs[0]
}
