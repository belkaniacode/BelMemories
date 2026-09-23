package index

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"belmemories/internal/hashing"
	"belmemories/internal/layout"
	"belmemories/internal/media"
)

// ReindexProgress reports reindexing state.
type ReindexProgress struct {
	Files   int    `json:"files"`
	Added   int    `json:"added"`
	Errors  int    `json:"errors"`
	Done    bool   `json:"done"`
	Current string `json:"current"`
}

// NeedsReindex reports whether archive content folders exist but the index is
// empty (e.g. the index was deleted or the archive was created by hand).
func (d *DB) NeedsReindex() bool {
	var n int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM files`).Scan(&n); err != nil || n > 0 {
		return false
	}
	for _, dir := range layout.ContentDirs() {
		entries, err := os.ReadDir(filepath.Join(d.root, dir))
		if err == nil && len(entries) > 0 {
			return true
		}
	}
	return false
}

// Reindex hashes every media file under the archive content folders and adds
// the ones missing from the index. Existing records are kept.
func (d *DB) Reindex(ctx context.Context, progress func(ReindexProgress)) (ReindexProgress, error) {
	d.log.Info("reindex started")
	start := time.Now()
	var p ReindexProgress
	var batch []FileRecord
	last := time.Time{}

	flush := func() error {
		if err := d.InsertBatch(batch); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	for _, top := range layout.ContentDirs() {
		base := filepath.Join(d.root, top)
		err := filepath.WalkDir(base, func(path string, de fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return fs.SkipDir
				}
				p.Errors++
				return nil
			}
			if de.IsDir() || !de.Type().IsRegular() {
				return nil
			}
			info, err := de.Info()
			if err != nil {
				p.Errors++
				return nil
			}
			kind := media.Detect(path, info.Size())
			if kind == media.Other {
				return nil
			}
			p.Files++
			p.Current = path
			rel, _ := filepath.Rel(d.root, path)
			if exists, _ := d.RelPathExists(rel); exists {
				return nil
			}
			sum, err := hashing.HashFile(ctx, path, nil)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				p.Errors++
				return nil
			}
			cat, year := categoryYearFromRel(rel)
			batch = append(batch, FileRecord{
				Hash: sum, Size: info.Size(), RelPath: rel, Kind: string(kind),
				Category: string(cat), Year: year, DateSource: "reindex",
				OrigName: filepath.Base(path), AddedAt: info.ModTime(),
			})
			p.Added++
			if len(batch) >= 200 {
				if err := flush(); err != nil {
					return err
				}
			}
			if progress != nil && time.Since(last) > 100*time.Millisecond {
				last = time.Now()
				progress(p)
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			_ = flush()
			d.log.Error("reindex aborted", "err", err)
			return p, err
		}
	}
	if err := flush(); err != nil {
		return p, err
	}
	p.Done = true
	p.Current = ""
	if progress != nil {
		progress(p)
	}
	d.log.Info("reindex finished", "files", p.Files, "added", p.Added, "errors", p.Errors, "took", time.Since(start).String())
	return p, nil
}

// categoryYearFromRel infers category and year from "Фото/2019/x.jpg" or
// "Без даты/Видео/x.mp4".
func categoryYearFromRel(rel string) (layout.Category, int) {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) == 0 {
		return layout.CatPhoto, 0
	}
	top := parts[0]
	year := 0
	if top == layout.DirNoDate && len(parts) > 1 {
		top = parts[1]
	} else if len(parts) > 1 {
		year, _ = strconv.Atoi(parts[1])
	}
	switch top {
	case layout.DirVideos:
		return layout.CatVideo, year
	case layout.DirPictures:
		return layout.CatPicture, year
	default:
		return layout.CatPhoto, year
	}
}
