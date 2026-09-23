package metadata

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"sync"
	"time"

	mp4 "github.com/abema/go-mp4"

	"memoryarchive/internal/logging"
)

var isoBMFFExts = map[string]bool{"mp4": true, "m4v": true, "mov": true, "3gp": true, "3g2": true}

// mp4Epoch is the ISO-BMFF time origin (1904-01-01 UTC).
var mp4Epoch = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)

// fromVideo reads the creation date from container metadata.
func fromVideo(path, ext string) (time.Time, bool) {
	if isoBMFFExts[ext] {
		if t, ok := fromMvhd(path); ok {
			return t, true
		}
	}
	return fromFFprobe(path)
}

// fromMvhd reads moov/mvhd creation_time without reading the media data.
func fromMvhd(path string) (t time.Time, ok bool) {
	log := logging.Component("metadata")
	f, err := os.Open(path)
	if err != nil {
		log.Warn("cannot open video", "path", path, "err", err)
		return t, false
	}
	defer f.Close()
	defer func() {
		if r := recover(); r != nil {
			log.Warn("mp4 parser panic", "path", path, "panic", r)
			ok = false
		}
	}()

	boxes, err := mp4.ExtractBoxWithPayload(f, nil, mp4.BoxPath{mp4.BoxTypeMoov(), mp4.BoxTypeMvhd()})
	if err != nil || len(boxes) == 0 {
		log.Debug("no mvhd", "path", path, "err", err)
		return t, false
	}
	mvhd, isMvhd := boxes[0].Payload.(*mp4.Mvhd)
	if !isMvhd {
		return t, false
	}
	secs := mvhd.GetCreationTime()
	if secs == 0 {
		return t, false
	}
	t = mp4Epoch.Add(time.Duration(secs) * time.Second)
	if !validDate(t) {
		log.Debug("mvhd date out of range", "path", path, "time", t)
		return time.Time{}, false
	}
	return t, true
}

var (
	ffprobeOnce sync.Once
	ffprobePath string
)

func findFFprobe() string {
	ffprobeOnce.Do(func() {
		if p, err := exec.LookPath("ffprobe"); err == nil {
			ffprobePath = p
			logging.Component("metadata").Info("ffprobe found", "path", p)
		}
	})
	return ffprobePath
}

// fromFFprobe asks ffprobe (if installed) for format creation_time.
func fromFFprobe(path string) (time.Time, bool) {
	bin := findFFprobe()
	if bin == "" {
		return time.Time{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-v", "quiet", "-print_format", "json",
		"-show_entries", "format_tags=creation_time", path)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		logging.Component("metadata").Debug("ffprobe failed", "path", path, "err", err)
		return time.Time{}, false
	}
	var res struct {
		Format struct {
			Tags map[string]string `json:"tags"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return time.Time{}, false
	}
	raw := res.Format.Tags["creation_time"]
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, raw); err == nil && validDate(t) {
			return t, true
		}
	}
	return time.Time{}, false
}
