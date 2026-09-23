// Package scanner walks source directories and collects photo/video files.
package scanner

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"memoryarchive/internal/logging"
	"memoryarchive/internal/media"
)

// Item is a media file found during scanning.
type Item struct {
	Path    string     `json:"path"`
	Size    int64      `json:"size"`
	ModTime time.Time  `json:"modTime"`
	Kind    media.Kind `json:"kind"`
	Ext     string     `json:"ext"`
}

// Stats summarises a scan.
type Stats struct {
	Photos     int   `json:"photos"`
	Videos     int   `json:"videos"`
	PhotoBytes int64 `json:"photoBytes"`
	VideoBytes int64 `json:"videoBytes"`
	Skipped    int   `json:"skipped"`
	Errors     int   `json:"errors"`
	Dirs       int   `json:"dirs"`
	Done       bool  `json:"done"`
}

// Result is the full scan output.
type Result struct {
	Items      []Item        `json:"-"`
	Stats      Stats         `json:"stats"`
	ErrorPaths []string      `json:"errorPaths"`
	Duration   time.Duration `json:"duration"`
}

const maxErrorPaths = 1000

// Names of system/trash directories that are never scanned.
var skipDirNames = map[string]struct{}{
	"$recycle.bin":              {},
	"system volume information": {},
	"lost+found":                {},
	".memoryarchive":            {},
	"$windows.~bt":              {},
	"$windows.~ws":              {},
	".git":                      {},
	"node_modules":              {},
}

// Options for Scan.
type Options struct {
	Roots []string
	// Exclude lists absolute paths (e.g. the archive destination) to skip.
	Exclude []string
	// Progress is invoked at most ~10 times per second with running stats.
	Progress func(Stats)
}

// Scan walks all roots. Access errors are collected, not fatal. Cancelling ctx
// stops the walk and returns ctx.Err() together with partial results.
func Scan(ctx context.Context, opts Options) (*Result, error) {
	log := logging.Component("scanner")
	start := time.Now()
	log.Info("scan started", "roots", opts.Roots, "exclude", opts.Exclude)

	res := &Result{}
	excludes := make([]string, 0, len(opts.Exclude))
	for _, e := range opts.Exclude {
		if e == "" {
			continue
		}
		if abs, err := filepath.Abs(e); err == nil {
			excludes = append(excludes, filepath.Clean(abs))
		}
	}

	var mu sync.Mutex
	lastEmit := time.Time{}
	emit := func(force bool) {
		if opts.Progress == nil {
			return
		}
		if !force && time.Since(lastEmit) < 100*time.Millisecond {
			return
		}
		lastEmit = time.Now()
		opts.Progress(res.Stats)
	}
	addErr := func(path string, err error) {
		mu.Lock()
		defer mu.Unlock()
		res.Stats.Errors++
		if len(res.ErrorPaths) < maxErrorPaths {
			res.ErrorPaths = append(res.ErrorPaths, path+": "+err.Error())
		}
	}

	seenRoots := map[string]bool{}
	var walkErr error
	for _, root := range opts.Roots {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			addErr(root, err)
			continue
		}
		absRoot = filepath.Clean(absRoot)
		if seenRoots[absRoot] {
			continue
		}
		seenRoots[absRoot] = true

		err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if err != nil {
				log.Warn("cannot access path", "path", path, "err", err)
				addErr(path, err)
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if path != absRoot && shouldSkipDir(path, d.Name(), excludes) {
					log.Debug("skip dir", "path", path)
					return fs.SkipDir
				}
				if path == absRoot && isExcluded(path, excludes) {
					return fs.SkipDir
				}
				res.Stats.Dirs++
				emit(false)
				return nil
			}
			// Symlinks and special files are ignored (WalkDir does not follow symlinks).
			if !d.Type().IsRegular() {
				res.Stats.Skipped++
				return nil
			}
			info, err := d.Info()
			if err != nil {
				addErr(path, err)
				return nil
			}
			kind := media.Detect(path, info.Size())
			switch kind {
			case media.Photo:
				res.Stats.Photos++
				res.Stats.PhotoBytes += info.Size()
			case media.Video:
				res.Stats.Videos++
				res.Stats.VideoBytes += info.Size()
			default:
				res.Stats.Skipped++
				return nil
			}
			res.Items = append(res.Items, Item{
				Path:    path,
				Size:    info.Size(),
				ModTime: info.ModTime(),
				Kind:    kind,
				Ext:     media.Ext(path),
			})
			emit(false)
			return nil
		})
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				walkErr = err
				break
			}
			addErr(absRoot, err)
		}
	}

	res.Duration = time.Since(start)
	res.Stats.Done = walkErr == nil
	emit(true)
	log.Info("scan finished",
		"photos", res.Stats.Photos, "videos", res.Stats.Videos,
		"skipped", res.Stats.Skipped, "errors", res.Stats.Errors,
		"dirs", res.Stats.Dirs, "duration", res.Duration.String(), "cancelled", walkErr != nil)
	return res, walkErr
}

func shouldSkipDir(path, name string, excludes []string) bool {
	lower := strings.ToLower(name)
	if _, ok := skipDirNames[lower]; ok {
		return true
	}
	if strings.HasPrefix(lower, ".trash-") || lower == ".trash" || lower == ".trashes" {
		return true
	}
	return isExcluded(path, excludes)
}

func isExcluded(path string, excludes []string) bool {
	clean := filepath.Clean(path)
	for _, e := range excludes {
		if pathEqual(clean, e) {
			return true
		}
	}
	return false
}
