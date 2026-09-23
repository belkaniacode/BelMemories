package i18n

import "testing"

func TestFromLocale(t *testing.T) {
	cases := map[string]Lang{
		"ru_RU.UTF-8": RU, "ru-RU": RU, "ru": RU, "RU_ru": RU, "ru.UTF-8": RU,
		"en_US.UTF-8": EN, "de_DE": EN, "uk_UA": EN, "": EN, "C": EN, "rus": EN,
	}
	for loc, want := range cases {
		if got := FromLocale(loc); got != want {
			t.Errorf("FromLocale(%q) = %s, want %s", loc, got, want)
		}
	}
}
