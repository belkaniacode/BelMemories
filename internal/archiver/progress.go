package archiver

import (
	"sync"
	"sync/atomic"
	"time"
)

// Progress is a snapshot sent to the UI (≤10 per second).
type Progress struct {
	Phase          string  `json:"phase"` // running | paused | finishing | done
	Total          int64   `json:"total"`
	Processed      int64   `json:"processed"`
	TotalBytes     int64   `json:"totalBytes"`
	ProcessedBytes int64   `json:"processedBytes"`
	WrittenBytes   int64   `json:"writtenBytes"`
	BytesPerSec    float64 `json:"bytesPerSec"`
	ETASeconds     int64   `json:"etaSeconds"`
	Copied         int64   `json:"copied"`
	Duplicates     int64   `json:"duplicates"`
	Renamed        int64   `json:"renamed"`
	Errors         int64   `json:"errors"`
	NoDate         int64   `json:"noDate"`
	Photos         int64   `json:"photos"`
	Videos         int64   `json:"videos"`
	Pictures       int64   `json:"pictures"`
	Current        string  `json:"current"`
	ElapsedSeconds int64   `json:"elapsedSeconds"`
}

// counters are updated concurrently by pipeline stages.
type counters struct {
	processed, processedBytes, writtenBytes     atomic.Int64
	copied, duplicates, renamed, errors, noDate atomic.Int64
	photos, videos, pictures                    atomic.Int64
	current                                     atomic.Value // string
}

type tracker struct {
	c          counters
	total      int64
	totalBytes int64
	start      time.Time
	paused     func() bool

	mu        sync.Mutex
	lastBytes int64
	lastTime  time.Time
	rate      float64
}

func newTracker(total, totalBytes int64, paused func() bool) *tracker {
	t := &tracker{total: total, totalBytes: totalBytes, start: time.Now(), paused: paused}
	t.c.current.Store("")
	t.lastTime = t.start
	return t
}

func (t *tracker) snapshot(phase string) Progress {
	t.mu.Lock()
	now := time.Now()
	pb := t.c.processedBytes.Load()
	if dt := now.Sub(t.lastTime).Seconds(); dt >= 1 {
		inst := float64(pb-t.lastBytes) / dt
		if t.rate == 0 {
			t.rate = inst
		} else {
			t.rate = 0.7*t.rate + 0.3*inst // smoothed
		}
		t.lastBytes, t.lastTime = pb, now
	}
	rate := t.rate
	t.mu.Unlock()

	if phase == "running" && t.paused != nil && t.paused() {
		phase = "paused"
	}
	p := Progress{
		Phase:          phase,
		Total:          t.total,
		Processed:      t.c.processed.Load(),
		TotalBytes:     t.totalBytes,
		ProcessedBytes: pb,
		WrittenBytes:   t.c.writtenBytes.Load(),
		BytesPerSec:    rate,
		Copied:         t.c.copied.Load(),
		Duplicates:     t.c.duplicates.Load(),
		Renamed:        t.c.renamed.Load(),
		Errors:         t.c.errors.Load(),
		NoDate:         t.c.noDate.Load(),
		Photos:         t.c.photos.Load(),
		Videos:         t.c.videos.Load(),
		Pictures:       t.c.pictures.Load(),
		Current:        t.c.current.Load().(string),
		ElapsedSeconds: int64(now.Sub(t.start).Seconds()),
	}
	if rate > 0 && t.totalBytes > pb {
		p.ETASeconds = int64(float64(t.totalBytes-pb) / rate)
	}
	return p
}
