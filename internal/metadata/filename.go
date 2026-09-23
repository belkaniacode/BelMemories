package metadata

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Patterns are tried in order; each must capture year, month, day and may
// capture hour, minute, second.
var filenamePatterns = []*regexp.Regexp{
	// IMG_20190512_143000, VID_20190512_143000, PXL_20210101_120000123, 20190512_143000, 20190512-143000
	regexp.MustCompile(`(?:^|[^0-9])((?:19|20)\d{2})(\d{2})(\d{2})[_\-T ]?(\d{2})(\d{2})(\d{2})`),
	// WhatsApp: IMG-20190512-WA0001, VID-20190512-WA0001
	regexp.MustCompile(`(?i)(?:IMG|VID|PTT|AUD)-((?:19|20)\d{2})(\d{2})(\d{2})-WA\d+`),
	// Screenshot_2019-05-12-14-30-00, photo_2019-05-12_14-30-00 (Telegram), signal-2019-05-12-143000, 2019-05-12 14.30.00
	regexp.MustCompile(`(?:^|[^0-9])((?:19|20)\d{2})[-_.](\d{2})[-_.](\d{2})(?:[-_ T.]+(\d{2})[-_.:]?(\d{2})[-_.:]?(\d{2}))?`),
	// Bare 8-digit date: 20190512
	regexp.MustCompile(`(?:^|[^0-9])((?:19|20)\d{2})(\d{2})(\d{2})(?:[^0-9]|$)`),
}

// Unix epoch milliseconds (13 digits), e.g. 1557664200000.jpg from Android apps.
var epochMillis = regexp.MustCompile(`(?:^|[^0-9])(1[0-9]{12})(?:[^0-9]|$)`)

// FromFilename extracts a date encoded in the file name.
func FromFilename(path string) (time.Time, bool) {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	for _, re := range filenamePatterns {
		m := re.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		t, ok := buildDate(m[1:])
		if ok && validDate(t) {
			return t, true
		}
	}
	if m := epochMillis.FindStringSubmatch(name); m != nil {
		ms, err := strconv.ParseInt(m[1], 10, 64)
		if err == nil {
			t := time.UnixMilli(ms).UTC()
			if validDate(t) {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

func buildDate(parts []string) (time.Time, bool) {
	nums := make([]int, 6)
	for i := 0; i < 6 && i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return time.Time{}, false
		}
		nums[i] = n
	}
	y, mo, d, h, mi, s := nums[0], nums[1], nums[2], nums[3], nums[4], nums[5]
	if mo < 1 || mo > 12 || d < 1 || d > 31 || h > 23 || mi > 59 || s > 59 {
		return time.Time{}, false
	}
	t := time.Date(y, time.Month(mo), d, h, mi, s, 0, time.Local)
	// Reject normalised overflow such as 2019-02-31.
	if t.Day() != d || int(t.Month()) != mo {
		return time.Time{}, false
	}
	return t, true
}
