package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"memoryarchive/internal/archiver"
	"memoryarchive/internal/classify"
	"memoryarchive/internal/config"
	"memoryarchive/internal/drives"
	"memoryarchive/internal/index"
	"memoryarchive/internal/logging"
	"memoryarchive/internal/priority"
	"memoryarchive/internal/scanner"
)

// Event names shared with the frontend.
const (
	evScanProgress    = "scan:progress"
	evScanDone        = "scan:done"
	evArchiveProgress = "archive:progress"
	evArchiveDone     = "archive:done"
	evArchiveError    = "archive:error"
	evReindexProgress = "reindex:progress"
	evReindexDone     = "reindex:done"
	evClassifierReady = "classifier:ready"
	evConfirmClose    = "app:confirm-close"
)

// App is the Wails-bound application facade. All long operations run in
// goroutines and report through events, so the UI never blocks.
type App struct {
	ctx      context.Context
	log      *slog.Logger
	logPath  string
	settings *config.Store

	mu            sync.Mutex
	scanCancel    context.CancelFunc
	scan          *scanner.Result
	scanRoots     []string
	archiveCancel context.CancelFunc
	archiveDone   chan struct{}
	runner        *archiver.Runner
	classifier    *classify.Classifier
	classifierMu  sync.Mutex
	closing       bool
}

// NewApp creates the application facade.
func NewApp(logPath string) *App {
	return &App{
		log:      logging.Component("app"),
		logPath:  logPath,
		settings: config.NewStore(""),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.log.Info("app started", "version", version, "os", goruntime.GOOS, "arch", goruntime.GOARCH, "log", a.logPath)
	// Load the model in the background so the window appears immediately.
	go func() {
		st := a.getClassifier().Status()
		runtime.EventsEmit(a.ctx, evClassifierReady, st)
	}()
}

func (a *App) shutdown(ctx context.Context) {
	a.cancelAll(30 * time.Second)
	a.classifierMu.Lock()
	a.classifier.Close()
	a.classifierMu.Unlock()
	a.log.Info("app shutdown")
}

// beforeClose asks the UI to confirm while archiving is in progress.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	a.mu.Lock()
	busy := a.archiveCancel != nil && !a.closing
	a.mu.Unlock()
	if busy {
		a.log.Info("close requested during archiving, asking user")
		runtime.EventsEmit(a.ctx, evConfirmClose)
		return true
	}
	return false
}

// ConfirmClose stops the running job gracefully and quits.
func (a *App) ConfirmClose() {
	a.log.Info("close confirmed by user")
	a.mu.Lock()
	a.closing = true
	a.mu.Unlock()
	go func() {
		a.cancelAll(30 * time.Second)
		runtime.Quit(a.ctx)
	}()
}

// cancelAll cancels scan/archive and waits (bounded) for the archive to stop.
func (a *App) cancelAll(wait time.Duration) {
	a.mu.Lock()
	if a.scanCancel != nil {
		a.scanCancel()
	}
	cancel, done := a.archiveCancel, a.archiveDone
	a.mu.Unlock()
	if cancel != nil {
		cancel()
		select {
		case <-done:
		case <-time.After(wait):
			a.log.Warn("archive did not stop in time")
		}
	}
}

// getClassifier lazily builds the classifier from current settings.
func (a *App) getClassifier() *classify.Classifier {
	a.classifierMu.Lock()
	defer a.classifierMu.Unlock()
	if a.classifier == nil {
		st := a.settings.Load()
		prof := priority.ProfileFor(st.Load, 0)
		a.classifier = classify.New(classify.Options{
			ModelDir: st.ModelDir, Threads: prof.CPUWorkers, Threshold: st.ClipThreshold,
		})
	}
	return a.classifier
}

// ---- Settings ----

// GetSettings returns persisted settings.
func (a *App) GetSettings() config.Settings { return a.settings.Load() }

// SaveSettings persists settings. Changing the model dir or threshold reloads
// the classifier on next use.
func (a *App) SaveSettings(s config.Settings) error {
	old := a.settings.Load()
	if err := a.settings.Save(s); err != nil {
		a.log.Error("save settings failed", "err", err)
		return err
	}
	a.mu.Lock()
	archiving := a.archiveCancel != nil
	a.mu.Unlock()
	// The running job keeps using the current classifier; reload afterwards.
	if !archiving && (old.ModelDir != s.ModelDir || old.ClipThreshold != s.ClipThreshold) {
		a.classifierMu.Lock()
		a.classifier.Close()
		a.classifier = nil
		a.classifierMu.Unlock()
	}
	return nil
}

// ClassifierStatus reports whether the neural network is active.
func (a *App) ClassifierStatus() classify.Status { return a.getClassifier().Status() }

// ---- Sources ----

// ListDrives returns mounted disks.
func (a *App) ListDrives() []drives.Drive { return drives.List() }

// PickFolder opens a native folder dialog; returns "" if cancelled.
func (a *App) PickFolder(title string) (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: title, CanCreateDirectories: true})
	if err != nil {
		a.log.Error("folder dialog failed", "err", err)
	}
	return dir, err
}

// ScanSummary is sent with scan:done.
type ScanSummary struct {
	Stats      scanner.Stats `json:"stats"`
	ErrorPaths []string      `json:"errorPaths"`
	Seconds    float64       `json:"seconds"`
	Cancelled  bool          `json:"cancelled"`
	Roots      []string      `json:"roots"`
}

// StartScan scans roots in the background (events scan:progress/scan:done).
func (a *App) StartScan(roots []string) error {
	if len(roots) == 0 {
		return errors.New("не выбрано ни одного источника")
	}
	a.mu.Lock()
	if a.scanCancel != nil {
		a.mu.Unlock()
		return errors.New("сканирование уже идёт")
	}
	if a.archiveCancel != nil {
		a.mu.Unlock()
		return errors.New("идёт архивация")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.scanCancel = cancel
	a.scan = nil
	a.mu.Unlock()

	st := a.settings.Load()
	st.RecentSources = mergeRecent(roots, st.RecentSources)
	_ = a.settings.Save(st)
	a.log.Info("StartScan", "roots", roots)

	go func() {
		res, err := scanner.Scan(ctx, scanner.Options{
			Roots:    roots,
			Progress: func(s scanner.Stats) { runtime.EventsEmit(a.ctx, evScanProgress, s) },
		})
		a.mu.Lock()
		a.scanCancel = nil
		a.scan, a.scanRoots = res, roots
		a.mu.Unlock()
		cancel()

		sum := ScanSummary{Stats: res.Stats, Seconds: res.Duration.Seconds(), Cancelled: err != nil, Roots: roots}
		sum.ErrorPaths = res.ErrorPaths
		if len(sum.ErrorPaths) > 200 {
			sum.ErrorPaths = sum.ErrorPaths[:200]
		}
		runtime.EventsEmit(a.ctx, evScanDone, sum)
	}()
	return nil
}

// CancelScan stops a running scan.
func (a *App) CancelScan() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.scanCancel != nil {
		a.log.Info("CancelScan")
		a.scanCancel()
	}
}

// ---- Destination ----

// DestinationInfo extends archive info with the estimate for the last scan.
type DestinationInfo struct {
	archiver.ArchiveInfo
	NeededBytes  int64 `json:"neededBytes"`
	Enough       bool  `json:"enough"`
	InsideSource bool  `json:"insideSource"`
}

// GetDestinationInfo inspects a destination folder (read-only).
func (a *App) GetDestinationInfo(root string) DestinationInfo {
	info := DestinationInfo{ArchiveInfo: archiver.GetArchiveInfo(root)}
	a.mu.Lock()
	scan, roots := a.scan, a.scanRoots
	a.mu.Unlock()
	if scan != nil && info.Exists {
		items := filterOutside(scan.Items, root)
		info.NeededBytes = archiver.EstimateNeeded(root, items)
		info.Enough = uint64(info.NeededBytes) <= info.FreeBytes
		for _, r := range roots {
			if isInside(r, root) {
				info.InsideSource = true
			}
		}
	}
	return info
}

// ArchiveRequest starts a run.
type ArchiveRequest struct {
	Root   string           `json:"root"`
	DryRun bool             `json:"dryRun"`
	Verify bool             `json:"verify"`
	Load   config.LoadLevel `json:"load"`
}

// StartArchive archives the last scan into req.Root (events archive:*).
func (a *App) StartArchive(req ArchiveRequest) error {
	a.mu.Lock()
	if a.archiveCancel != nil {
		a.mu.Unlock()
		return errors.New("архивация уже идёт")
	}
	if a.scan == nil || len(a.scan.Items) == 0 {
		a.mu.Unlock()
		return errors.New("сначала выполните сканирование")
	}
	if req.Root == "" {
		a.mu.Unlock()
		return errors.New("не выбрана папка назначения")
	}
	items := filterOutside(a.scan.Items, req.Root)
	sources := append([]string(nil), a.scanRoots...)
	ctx, cancel := context.WithCancel(a.ctx)
	done := make(chan struct{})
	a.archiveCancel, a.archiveDone = cancel, done
	a.mu.Unlock()

	st := a.settings.Load()
	st.LastDestination, st.Load, st.VerifyAfterCopy = req.Root, req.Load, req.Verify
	_ = a.settings.Save(st)

	prof := priority.ProfileFor(req.Load, st.BytesPerSecLimit)
	a.log.Info("StartArchive", "root", req.Root, "files", len(items), "dryRun", req.DryRun, "load", req.Load)

	go func() {
		defer func() {
			a.mu.Lock()
			a.archiveCancel, a.archiveDone, a.runner = nil, nil, nil
			a.mu.Unlock()
			cancel()
			close(done)
		}()
		runner := archiver.NewRunner(a.getClassifier())
		a.mu.Lock()
		a.runner = runner
		a.mu.Unlock()

		rep, err := runner.Run(ctx, archiver.Plan{
			Items: items, Sources: sources, ArchiveRoot: req.Root,
			Options: archiver.Options{Profile: prof, Verify: req.Verify, DryRun: req.DryRun, OtherArchives: st.OtherArchives},
		}, func(p archiver.Progress) { runtime.EventsEmit(a.ctx, evArchiveProgress, p) })
		if err != nil {
			a.log.Error("archive failed", "err", err)
			runtime.EventsEmit(a.ctx, evArchiveError, err.Error())
			return
		}
		runtime.EventsEmit(a.ctx, evArchiveDone, rep)
	}()
	return nil
}

// PauseArchive pauses the running job.
func (a *App) PauseArchive() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.runner != nil {
		a.runner.Pause()
	}
}

// ResumeArchive resumes the running job.
func (a *App) ResumeArchive() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.runner != nil {
		a.runner.Resume()
	}
}

// CancelArchive stops the running job (the current file is rolled back).
func (a *App) CancelArchive() {
	a.mu.Lock()
	cancel := a.archiveCancel
	a.mu.Unlock()
	if cancel != nil {
		a.log.Info("CancelArchive")
		cancel()
	}
}

// Reindex rebuilds the index of an archive (events reindex:*).
func (a *App) Reindex(root string) error {
	a.log.Info("Reindex", "root", root)
	db, err := index.Open(root, false)
	if err != nil {
		return err
	}
	go func() {
		defer db.Close()
		p, err := db.Reindex(a.ctx, func(p index.ReindexProgress) { runtime.EventsEmit(a.ctx, evReindexProgress, p) })
		if err != nil {
			a.log.Error("reindex failed", "err", err)
		}
		runtime.EventsEmit(a.ctx, evReindexDone, p)
	}()
	return nil
}

// OpenPath opens a folder or file with the system file manager/viewer.
func (a *App) OpenPath(path string) error {
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", filepath.Clean(path))
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		a.log.Error("open path failed", "path", path, "err", err)
		return fmt.Errorf("не удалось открыть %s: %w", path, err)
	}
	go cmd.Wait()
	return nil
}

// LogFrontendError records UI errors in the application log.
func (a *App) LogFrontendError(msg string) {
	a.log.Warn("frontend error", "msg", msg)
}

// ---- helpers ----

func mergeRecent(add, old []string) []string {
	out := append([]string(nil), add...)
	for _, o := range old {
		dup := false
		for _, n := range out {
			if n == o {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, o)
		}
	}
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

// isInside reports whether path is dir itself or inside it.
func isInside(path, dir string) bool {
	p, err1 := filepath.Abs(path)
	d, err2 := filepath.Abs(dir)
	if err1 != nil || err2 != nil {
		return false
	}
	if goruntime.GOOS == "windows" {
		p, d = strings.ToLower(p), strings.ToLower(d)
	}
	rel, err := filepath.Rel(d, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// filterOutside drops items located inside the archive (never re-archive the archive).
func filterOutside(items []scanner.Item, root string) []scanner.Item {
	out := make([]scanner.Item, 0, len(items))
	for _, it := range items {
		if !isInside(it.Path, root) {
			out = append(out, it)
		}
	}
	return out
}
