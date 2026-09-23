package archiver

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"belmemories/internal/fsutil"
	"belmemories/internal/hashing"
	"belmemories/internal/i18n"
	"belmemories/internal/index"
	"belmemories/internal/layout"
	"belmemories/internal/logging"
	"belmemories/internal/scanner"
)

// archiveSet is the primary (writable) index plus read-only indexes of other
// archive disks used for cross-disk deduplication. primary may be nil in a
// dry run on a brand-new destination.
type archiveSet struct {
	primary *index.DB
	others  []*index.DB
}

// openOthers opens read-only indexes of the other archives that are
// currently connected. Disconnected or non-archive roots are skipped quietly:
// the list is collected automatically and often names unplugged disks.
func openOthers(roots []string, primaryRoot string) (dbs []*index.DB) {
	log := logging.Component("archiver")
	primaryAbs, _ := filepath.Abs(primaryRoot)
	for _, r := range roots {
		abs, err := filepath.Abs(r)
		if err != nil || samePath(abs, primaryAbs) {
			continue
		}
		if !index.Exists(abs) {
			log.Debug("known archive not connected", "root", abs)
			continue
		}
		db, err := index.Open(abs, true)
		if err != nil {
			log.Warn("other archive unreadable", "root", abs, "err", err)
			continue
		}
		dbs = append(dbs, db)
	}
	if len(dbs) > 0 {
		names := make([]string, len(dbs))
		for i, d := range dbs {
			names[i] = d.Root()
		}
		log.Info("[FIX] other archives connected for dedup", "roots", names)
	}
	return dbs
}

func (s *archiveSet) all() []*index.DB {
	var out []*index.DB
	if s.primary != nil {
		out = append(out, s.primary)
	}
	return append(out, s.others...)
}

// hasSize reports whether any archive holds a file of this size.
func (s *archiveSet) hasSize(size int64) (bool, error) {
	for _, db := range s.all() {
		ok, err := db.HasSize(size)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// lookup returns "<root>/<rel>" of an archived file with this hash. A record
// only counts when the file is still on disk: files deleted from the archive by
// hand must be archived again, so their stale records are dropped.
func (s *archiveSet) lookup(sum hashing.Sum) (string, bool, error) {
	for _, db := range s.all() {
		rel, ok, err := db.LookupHash(sum)
		if err != nil {
			return "", false, err
		}
		if !ok {
			continue
		}
		path := filepath.Join(db.Root(), filepath.FromSlash(rel))
		if st, err := os.Stat(path); err == nil && st.Mode().IsRegular() {
			return path, true, nil
		} else if !errors.Is(err, fs.ErrNotExist) && err != nil {
			// Unreadable (permissions, I/O): keep treating it as archived
			// rather than risk a second copy.
			return path, true, nil
		}
		log := logging.Component("archiver")
		log.Info("[FIX] indexed file missing on disk, will archive again", "path", path)
		if !db.ReadOnly() {
			if err := db.DeleteStale(sum, rel); err != nil {
				log.Warn("[FIX] cannot drop stale index record", "rel", rel, "err", err)
			}
		}
	}
	return "", false, nil
}

// purgeStale removes primary-index records whose file is gone from disk, for
// every size among items (only those records can match a scanned file). It
// runs single-threaded before the pipeline, so it cannot race with writes.
func (s *archiveSet) purgeStale(items []scanner.Item) {
	if s.primary == nil || s.primary.ReadOnly() {
		return
	}
	log := logging.Component("archiver")
	sizes := map[int64]bool{}
	removed := 0
	for _, it := range items {
		if sizes[it.Size] {
			continue
		}
		sizes[it.Size] = true
		rels, err := s.primary.PathsWithSize(it.Size)
		if err != nil {
			log.Warn("stale check failed", "err", err)
			return
		}
		for _, rel := range rels {
			_, err := os.Stat(filepath.Join(s.primary.Root(), filepath.FromSlash(rel)))
			if !errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err := s.primary.DeleteByRelPath(rel); err != nil {
				log.Warn("cannot drop stale index record", "rel", rel, "err", err)
				continue
			}
			removed++
		}
	}
	if removed > 0 {
		log.Info("[FIX] stale index records dropped (files deleted from the archive)", "count", removed)
	}
}

func (s *archiveSet) closeOthers() {
	for _, db := range s.others {
		db.Close()
	}
}

// ArchiveInfo describes a destination for the UI.
type ArchiveInfo struct {
	Root         string      `json:"root"`
	Exists       bool        `json:"exists"`
	IsArchive    bool        `json:"isArchive"`
	FreeBytes    uint64      `json:"freeBytes"`
	Stats        index.Stats `json:"stats"`
	NeedsReindex bool        `json:"needsReindex"`
	Error        string      `json:"error,omitempty"`
}

// GetArchiveInfo inspects a destination without modifying it. Stats describe
// the files actually present in the content folders, not the index: the user
// may have deleted or moved files by hand.
func GetArchiveInfo(root string) ArchiveInfo {
	info := ArchiveInfo{Root: root}
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		info.Error = i18n.Pick("папка недоступна", "folder is not accessible")
		return info
	}
	info.Exists = true
	if free, err := fsutil.FreeSpace(root); err == nil {
		info.FreeBytes = free
	}
	info.Stats = diskStats(root)
	logging.Component("archiver").Info("[FIX] destination on disk", "root", root,
		"files", info.Stats.Total.Count, "bytes", info.Stats.Total.Bytes)
	if !index.Exists(root) {
		return info
	}
	info.IsArchive = true
	db, err := index.Open(root, true)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	defer db.Close()
	info.NeedsReindex = db.NeedsReindex()
	return info
}

// diskStats counts files in the archive content folders by category and year.
func diskStats(root string) index.Stats {
	st := index.Stats{ByCategory: map[string]index.KindStats{}}
	years := map[int]bool{}
	cats := map[string]layout.Category{
		layout.CategoryDir(layout.CatPhoto):   layout.CatPhoto,
		layout.CategoryDir(layout.CatVideo):   layout.CatVideo,
		layout.CategoryDir(layout.CatPicture): layout.CatPicture,
	}
	for _, top := range layout.ContentDirs() {
		base := filepath.Join(root, top)
		_ = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || strings.HasPrefix(d.Name(), ".") {
				return nil
			}
			fi, err := d.Info()
			if err != nil {
				return nil
			}
			// <Категория>/<год>/… or Без даты/<Категория>/…
			parts := strings.Split(filepath.ToSlash(strings.TrimPrefix(p, base+string(filepath.Separator))), "/")
			cat, ok := cats[top]
			if !ok {
				if cat, ok = cats[parts[0]]; !ok || len(parts) == 1 {
					cat = layout.CatPhoto
				}
			} else if ok && len(parts) > 1 {
				if y, err := strconv.Atoi(parts[0]); err == nil && y > 0 {
					years[y] = true
				}
			}
			ks := st.ByCategory[string(cat)]
			ks.Count++
			ks.Bytes += fi.Size()
			st.ByCategory[string(cat)] = ks
			st.Total.Count++
			st.Total.Bytes += fi.Size()
			return nil
		})
	}
	for y := range years {
		st.Years = append(st.Years, y)
	}
	slices.Sort(st.Years)
	return st
}

// EstimateNeeded returns bytes and count of scanned files that will probably
// be written: a file is skipped only when a file of the same size is catalogued
// AND still present on disk. The safety reserve (ReserveBytes) is not included.
func EstimateNeeded(root string, items []scanner.Item) (bytes int64, files int) {
	var db *index.DB
	if index.Exists(root) {
		db, _ = index.Open(root, true)
		defer db.Close()
	}
	present := map[int64]bool{}
	for _, it := range items {
		if db != nil {
			ok, seen := present[it.Size]
			if !seen {
				ok = sizeOnDisk(db, it.Size)
				present[it.Size] = ok
			}
			if ok {
				continue
			}
		}
		bytes += it.Size
		files++
	}
	return bytes, files
}

// sizeOnDisk reports whether some catalogued file of this size still exists.
func sizeOnDisk(db *index.DB, size int64) bool {
	rels, err := db.PathsWithSize(size)
	if err != nil {
		return false
	}
	for _, rel := range rels {
		if fi, err := os.Stat(filepath.Join(db.Root(), filepath.FromSlash(rel))); err == nil && fi.Size() == size {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
