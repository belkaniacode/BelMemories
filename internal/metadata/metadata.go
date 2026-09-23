// Package metadata determines when a photo or video was taken.
//
// The chain is: EXIF → container metadata (MP4/MOV, optional ffprobe) →
// date pattern in the file name → file modification time. Every result
// records which source produced it.
package metadata

import (
	"time"

	"memoryarchive/internal/logging"
	"memoryarchive/internal/media"
	"memoryarchive/internal/scanner"
)

// Source describes where the date came from.
type Source string

const (
	SourceExif      Source = "exif"
	SourceContainer Source = "container"
	SourceFilename  Source = "filename"
	SourceMtime     Source = "mtime"
	SourceNone      Source = "none"
)

// DateInfo is the extracted date plus image facts useful for classification.
type DateInfo struct {
	Time   time.Time `json:"time"`
	Source Source    `json:"source"`
	// Width/Height from EXIF when available (0 if unknown).
	Width  int `json:"width"`
	Height int `json:"height"`
	// HasCameraInfo is true when EXIF names a camera make or model.
	HasCameraInfo bool   `json:"hasCameraInfo"`
	HasExif       bool   `json:"hasExif"`
	Make          string `json:"make"`
	Model         string `json:"model"`
	Software      string `json:"software"`
}

// Year returns the year or 0 when there is no date.
func (d DateInfo) Year() int {
	if d.Source == SourceNone || d.Time.IsZero() {
		return 0
	}
	return d.Time.Year()
}

var minValid = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

// now is replaceable in tests.
var now = time.Now

// validDate reports whether t is a plausible capture date.
func validDate(t time.Time) bool {
	if t.IsZero() {
		return false
	}
	return !t.Before(minValid) && !t.After(now().Add(24*time.Hour))
}

// Extract determines the capture date for a scanned item. It never fails:
// parse errors fall through to the next source.
func Extract(item scanner.Item) DateInfo {
	log := logging.Component("metadata")
	var info DateInfo

	switch item.Kind {
	case media.Photo:
		info = fromImage(item.Path, item.Ext)
	case media.Video:
		if t, ok := fromVideo(item.Path, item.Ext); ok {
			info.Time, info.Source = t, SourceContainer
		}
	}

	if info.Source == "" || info.Source == SourceNone || !validDate(info.Time) {
		if t, ok := FromFilename(item.Path); ok {
			info.Time, info.Source = t, SourceFilename
		} else if validDate(item.ModTime) {
			info.Time, info.Source = item.ModTime, SourceMtime
		} else {
			info.Time, info.Source = time.Time{}, SourceNone
		}
	}

	log.Debug("date extracted", "path", item.Path, "source", info.Source, "time", info.Time)
	return info
}
