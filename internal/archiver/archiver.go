// Package archiver runs the archiving pipeline: analyse (date + category),
// deduplicate by content hash and copy safely into
// <archive>/<Фото|Видео|Картинки>/<year>/.
package archiver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"belmemories/internal/classify"
	"belmemories/internal/fsutil"
	"belmemories/internal/hashing"
	"belmemories/internal/index"
	"belmemories/internal/layout"
	"belmemories/internal/logging"
	"belmemories/internal/priority"
	"belmemories/internal/scanner"
)

// ReserveBytes is always kept free on the destination disk.
const ReserveBytes int64 = 1 << 30

// ErrNoSpace stops a run when the destination is (almost) full.
var ErrNoSpace = errors.New("not enough free space on destination")

// Options configure a run.
type Options struct {
	Profile priority.Profile
	Verify  bool
	DryRun  bool
	// OtherArchives are further archive roots checked for duplicates
	// (read-only); disconnected ones are skipped.
	OtherArchives []string
	// ReportDir overrides where dry-run reports go (never the destination).
	ReportDir string
}

// Plan is the input of a run.
type Plan struct {
	Items       []scanner.Item
	Sources     []string
	ArchiveRoot string
	Options     Options
}

// Runner executes one archiving run with pause/resume support.
type Runner struct {
	classifier *classify.Classifier
	log        *slog.Logger

	mu     sync.Mutex
	cond   *sync.Cond
	paused bool
}

// NewRunner creates a runner. classifier may be nil (rules-free: everything
// that is not a video goes to Фото).
func NewRunner(c *classify.Classifier) *Runner {
	r := &Runner{classifier: c, log: logging.Component("archiver")}
	r.cond = sync.NewCond(&r.mu)
	return r
}

// Pause suspends processing after the current files.
func (r *Runner) Pause() {
	r.mu.Lock()
	r.paused = true
	r.mu.Unlock()
	r.log.Info("paused")
}

// Resume continues a paused run.
func (r *Runner) Resume() {
	r.mu.Lock()
	r.paused = false
	r.mu.Unlock()
	r.cond.Broadcast()
	r.log.Info("resumed")
}

// IsPaused reports the pause state.
func (r *Runner) IsPaused() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.paused
}

// waitIfPaused blocks while paused; returns false if ctx was cancelled.
func (r *Runner) waitIfPaused(ctx context.Context) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for r.paused && ctx.Err() == nil {
		r.cond.Wait()
	}
	return ctx.Err() == nil
}

// job flows through the pipeline stages.
type job struct {
	item     scanner.Item
	dateSrc  string
	year     int
	category layout.Category
	method   string
	hash     hashing.Sum // set when pre-hashed in the dedup stage
}

// Run executes the plan. Progress snapshots are sent to onProgress at most
// 10 times per second (and once at the end).
func (r *Runner) Run(ctx context.Context, plan Plan, onProgress func(Progress)) (*Report, error) {
	log := r.log.With("root", plan.ArchiveRoot, "dryRun", plan.Options.DryRun)
	opts := plan.Options
	root := plan.ArchiveRoot
	started := time.Now()
	rep := &Report{Root: root, Sources: plan.Sources, DryRun: opts.DryRun, Started: started}

	// Wake paused workers on cancellation.
	stopWake := context.AfterFunc(ctx, func() { r.cond.Broadcast() })
	defer stopWake()

	set := &archiveSet{}
	var err error
	if opts.DryRun {
		if index.Exists(root) {
			set.primary, err = index.Open(root, true)
		}
	} else {
		set.primary, err = index.Open(root, false)
	}
	if err != nil {
		return nil, fmt.Errorf("open archive index: %w", err)
	}
	defer set.primary.Close()
	set.others = openOthers(opts.OtherArchives, root)
	defer set.closeOthers()

	tmpDir := layout.MetaPath(root, "tmp")
	if !opts.DryRun {
		if _, err := fsutil.CleanupTmp(tmpDir); err != nil {
			log.Warn("tmp cleanup failed", "err", err)
		}
		if _, err := set.primary.RecoverWritten(); err != nil {
			log.Warn("journal recovery failed", "err", err)
		}
		if _, err := set.primary.FailStale(); err != nil {
			log.Warn("journal cleanup failed", "err", err)
		}
		// Before workers start: drop records of files deleted from the
		// archive by hand, so a new file taking such a path mid-run cannot
		// be mistaken for the old one.
		set.purgeStale(plan.Items)
		rep.RunID, err = set.primary.StartRun(plan.Sources)
		if err != nil {
			return nil, err
		}
	}

	stamp := started.Format("2006-01-02_15-04-05")
	reportDir := layout.MetaPath(root, "reports")
	if opts.DryRun {
		reportDir = opts.ReportDir
		if reportDir == "" {
			reportDir = filepath.Join(logging.AppConfigDir(), "reports")
		}
		stamp += "_dry-run"
	}
	coll := newCollector(rep, filepath.Join(reportDir, stamp+".csv"))

	var totalBytes int64
	for _, it := range plan.Items {
		totalBytes += it.Size
	}
	tr := newTracker(int64(len(plan.Items)), totalBytes, r.IsPaused)

	runCtx, cancelRun := context.WithCancelCause(ctx)
	defer cancelRun(nil)

	priority.Apply(opts.Profile)
	defer priority.Restore()

	log.Info("run started", "files", len(plan.Items), "bytes", totalBytes, "runId", rep.RunID,
		"cpuWorkers", opts.Profile.CPUWorkers, "readers", opts.Profile.Readers)

	// Progress emitter.
	emitDone := make(chan struct{})
	go func() {
		defer close(emitDone)
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-t.C:
				if onProgress != nil {
					onProgress(tr.snapshot("running"))
				}
			}
		}
	}()

	p := &pipeline{
		r: r, set: set, opts: opts, root: root, tmpDir: tmpDir, runID: rep.RunID,
		tr: tr, coll: coll, log: log, cancel: cancelRun,
		limiter: hashing.NewLimiter(opts.Profile.BytesPerSec),
		seen:    map[hashing.Sum]string{},
	}
	p.run(runCtx, plan.Items)

	cause := context.Cause(runCtx)
	cancelRun(nil)
	<-emitDone
	coll.close()

	rep.Finished = time.Now()
	rep.Stats = tr.snapshot("done")
	switch {
	case errors.Is(cause, fsutil.ErrDiskFatal), errors.Is(cause, ErrNoSpace):
		rep.Status, rep.Message = StatusDiskError, cause.Error()
	case ctx.Err() != nil:
		rep.Status = StatusCancelled
	case cause != nil && !errors.Is(cause, context.Canceled):
		rep.Status, rep.Message = StatusError, cause.Error()
	default:
		rep.Status = StatusCompleted
	}

	rep.ReportPath = filepath.Join(reportDir, stamp+".json")
	if err := saveJSON(rep, rep.ReportPath); err != nil {
		log.Warn("cannot save report", "err", err)
		rep.ReportPath = ""
	}
	if !opts.DryRun && set.primary != nil {
		statsJSON, _ := json.Marshal(rep.Stats)
		if err := set.primary.FinishRun(rep.RunID, rep.Status, string(statsJSON)); err != nil {
			log.Warn("cannot finish run record", "err", err)
		}
	}
	if onProgress != nil {
		onProgress(rep.Stats)
	}

	log.Info("run finished", "status", rep.Status, "copied", rep.Stats.Copied,
		"duplicates", rep.Stats.Duplicates, "renamed", rep.Stats.Renamed,
		"errors", rep.Stats.Errors, "took", rep.Finished.Sub(started).String())
	if rep.Status == StatusDiskError {
		log.Error("run stopped because of destination disk problem", "err", rep.Message)
	}
	return rep, nil
}

// defaultWorkers returns a safe CPU worker count when the profile has none.
func defaultWorkers(n int) int {
	if n > 0 {
		return n
	}
	return max(1, min(4, runtime.NumCPU()/2))
}
