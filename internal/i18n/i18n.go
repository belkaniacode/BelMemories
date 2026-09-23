// Package i18n picks the UI language (Russian or English) from the OS locale.
//
// The language is fixed for the whole run: Russian when the system language is
// Russian, English for everything else. There is no user setting.
package i18n

import (
	"strings"
	"sync"
)

// Lang is a UI language code.
type Lang string

const (
	RU Lang = "ru"
	EN Lang = "en"
)

var (
	once    sync.Once
	current Lang
)

// Current returns the UI language, detected once from the OS.
func Current() Lang {
	once.Do(func() { current = FromLocale(systemLocale()) })
	return current
}

// FromLocale maps a locale string such as "ru_RU.UTF-8", "ru-RU" or "en_US"
// to a supported language.
func FromLocale(loc string) Lang {
	loc = strings.ToLower(strings.TrimSpace(loc))
	if loc == "ru" || strings.HasPrefix(loc, "ru_") || strings.HasPrefix(loc, "ru-") || strings.HasPrefix(loc, "ru.") {
		return RU
	}
	return EN
}

// Pick returns the Russian or the English text for the current language.
func Pick(ru, en string) string {
	if Current() == RU {
		return ru
	}
	return en
}
