package archiver

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"memoryarchive/internal/fsutil"
	"memoryarchive/internal/hashing"
	"memoryarchive/internal/index"
	"memoryarchive/internal/logging"
	"memoryarchive/internal/scanner"
)

// archiveSet is the primary (writable) index plus read-only indexes of other
// archive disks used for cross-disk deduplication. primary may be nil in a
// dry run on a brand-new destination.
type archiveSet struct {
	primary *index.DB
	others  []*index.DB
}

// openOthers opens read-only indexes of the other archives that are
// currently connected. Missing ones are reported, not fatal.
func openOthers(roots []string, primaryRoot string) (dbs []*index.DB, missing []string) {
	log := logging.Component("archiver")
	primaryAbs, _ := filepath.Abs(primaryRoot)
	for _, r := range roots {
		abs, err := filepath.Abs(r)
		if err != nil || samePath(abs, primaryAbs) {
			continue
		}
		db, err := index.Open(abs, true)
		if err != nil {
			log.Warn("other archive unavailable", "root", abs, "err", err)
			missing = append(missing, abs)
			continue
		}
		dbs = append(dbs, db)
	}
	if len(dbs) > 0 {
		names := make([]string, len(dbs))
		for i, d := range dbs {
			names[i] = d.Root()
		}
		log.Info("other archives connected for dedup", "roots", names)
	}
	return dbs, missing
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

// lookup returns "<root>/<rel>" of an archived file with this hash.
func (s *archiveSet) lookup(sum hashing.Sum) (string, bool, error) {
	for _, db := range s.all() {
		rel, ok, err := db.LookupHash(sum)
		if err != nil {
			return "", false, err
		}
		if ok {
			return filepath.Join(db.Root(), filepath.FromSlash(rel)), true, nil
		}
	}
	return "", false, nil
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

// GetArchiveInfo inspects a destination without modifying it.
func GetArchiveInfo(root string) ArchiveInfo {
	info := ArchiveInfo{Root: root}
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		info.Error = "папка недоступна"
		return info
	}
	info.Exists = true
	if free, err := fsutil.FreeSpace(root); err == nil {
		info.FreeBytes = free
	}
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
	if s, err := db.Stats(); err == nil {
		info.Stats = s
	}
	info.NeedsReindex = db.NeedsReindex()
	return info
}

// EstimateNeeded returns an upper bound of bytes that may be written: files
// whose size is already archived are likely duplicates and are not counted.
// A 1 GiB reserve is added.
func EstimateNeeded(root string, items []scanner.Item) int64 {
	var db *index.DB
	if index.Exists(root) {
		db, _ = index.Open(root, true)
		defer db.Close()
	}
	var total int64
	for _, it := range items {
		if db != nil {
			if ok, _ := db.HasSize(it.Size); ok {
				continue
			}
		}
		total += it.Size
	}
	return total + reserveBytes
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
