// Package index stores the per-archive catalogue of files and their hashes in
// SQLite at <archive>/.memoryarchive/index.db.
package index

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver

	"belmemories/internal/hashing"
	"belmemories/internal/layout"
	"belmemories/internal/logging"
)

// Journal states.
const (
	StatePending = "pending"
	StateWritten = "written"
	StateDone    = "done"
	StateFailed  = "failed"
)

// FileRecord is one archived file.
type FileRecord struct {
	Hash       hashing.Sum
	Size       int64
	RelPath    string // slash-separated, relative to archive root
	Kind       string
	Category   string
	Year       int
	DateSource string
	OrigName   string
	AddedAt    time.Time
}

// DB is an open archive index.
type DB struct {
	db       *sql.DB
	root     string
	readOnly bool
	log      *slog.Logger
}

// ErrNoIndex is returned when opening read-only and no index exists.
var ErrNoIndex = errors.New("archive index not found")

// Path returns the index file path for an archive root.
func Path(root string) string { return layout.MetaPath(root, "index.db") }

// Exists reports whether an archive index exists at root.
func Exists(root string) bool {
	_, err := os.Stat(Path(root))
	return err == nil
}

// Open opens (and, unless readOnly, creates/migrates) the index of an archive.
func Open(root string, readOnly bool) (*DB, error) {
	log := logging.Component("index").With("root", root)
	path := Path(root)
	if readOnly {
		if !Exists(root) {
			return nil, ErrNoIndex
		}
	} else if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create meta dir: %w", err)
	}

	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	if readOnly {
		q.Add("mode", "ro")
	} else {
		q.Add("_pragma", "journal_mode(WAL)")
		q.Add("_pragma", "synchronous(NORMAL)")
	}
	dsn := "file:" + filepath.ToSlash(path) + "?" + q.Encode()
	sdb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	sdb.SetMaxOpenConns(1)

	d := &DB{db: sdb, root: root, readOnly: readOnly, log: log}
	if !readOnly {
		if err := d.migrate(); err != nil {
			sdb.Close()
			return nil, err
		}
	} else if err := sdb.Ping(); err != nil {
		sdb.Close()
		return nil, fmt.Errorf("open index read-only: %w", err)
	}
	log.Info("index opened", "readOnly", readOnly)
	return d, nil
}

func (d *DB) migrate() error {
	if _, err := d.db.Exec(schemaSQL); err != nil {
		d.log.Error("schema migration failed", "err", err)
		return fmt.Errorf("migrate index: %w", err)
	}
	_, err := d.db.Exec(`INSERT INTO meta(key, value) VALUES('schema_version', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, schemaVersion)
	return err
}

// Close closes the database.
func (d *DB) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	return d.db.Close()
}

// Root returns the archive root.
func (d *DB) Root() string { return d.root }

// ReadOnly reports whether the index was opened read-only.
func (d *DB) ReadOnly() bool { return d.readOnly }

// HasSize reports whether any archived file has exactly this size.
func (d *DB) HasSize(size int64) (bool, error) {
	var one int
	err := d.db.QueryRow(`SELECT 1 FROM files WHERE size = ? LIMIT 1`, size).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		d.log.Error("HasSize failed", "err", err)
		return false, err
	}
	return true, nil
}

// LookupHash returns the archive-relative path of a file with this hash.
func (d *DB) LookupHash(sum hashing.Sum) (string, bool, error) {
	var rel string
	err := d.db.QueryRow(`SELECT rel_path FROM files WHERE hash = ?`, sum[:]).Scan(&rel)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		d.log.Error("LookupHash failed", "err", err)
		return "", false, err
	}
	return rel, true, nil
}

// PathsWithSize returns archive-relative paths of catalogued files of this size.
func (d *DB) PathsWithSize(size int64) ([]string, error) {
	rows, err := d.db.Query(`SELECT rel_path FROM files WHERE size = ?`, size)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var rel string
		if err := rows.Scan(&rel); err != nil {
			return nil, err
		}
		out = append(out, rel)
	}
	return out, rows.Err()
}

// RelPathExists reports whether a relative path is already catalogued.
func (d *DB) RelPathExists(rel string) (bool, error) {
	var one int
	err := d.db.QueryRow(`SELECT 1 FROM files WHERE rel_path = ?`, filepath.ToSlash(rel)).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// InsertBatch stores records in one transaction. Records whose hash already
// exists are ignored. A record owns its rel_path: the file was just written
// there, so a stale record of another hash at the same path (the old file was
// deleted by hand) is replaced instead of silently blocking the insert.
func (d *DB) InsertBatch(recs []FileRecord) error {
	if len(recs) == 0 {
		return nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO files
		(hash, size, rel_path, kind, category, year, date_source, orig_name, added_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()
	evict, err := tx.Prepare(`DELETE FROM files WHERE rel_path = ? AND hash != ?`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare evict: %w", err)
	}
	defer evict.Close()
	for _, r := range recs {
		rel := filepath.ToSlash(r.RelPath)
		if res, err := evict.Exec(rel, r.Hash[:]); err != nil {
			tx.Rollback()
			return fmt.Errorf("evict %s: %w", rel, err)
		} else if n, _ := res.RowsAffected(); n > 0 {
			d.log.Info("[FIX] stale record replaced at reused path", "rel", rel)
		}
		added := r.AddedAt
		if added.IsZero() {
			added = time.Now()
		}
		if _, err := stmt.Exec(r.Hash[:], r.Size, rel, r.Kind, r.Category,
			r.Year, r.DateSource, r.OrigName, added.Unix()); err != nil {
			tx.Rollback()
			d.log.Error("insert failed", "rel", r.RelPath, "err", err)
			return fmt.Errorf("insert %s: %w", r.RelPath, err)
		}
	}
	if err := tx.Commit(); err != nil {
		d.log.Error("commit failed", "err", err)
		return fmt.Errorf("commit: %w", err)
	}
	d.log.Debug("batch inserted", "count", len(recs))
	return nil
}

// DeleteStale removes the record of hash at rel (its file is gone). Both keys
// must match, so a record inserted meanwhile for another file at the same
// path is never removed.
func (d *DB) DeleteStale(sum hashing.Sum, rel string) error {
	_, err := d.db.Exec(`DELETE FROM files WHERE hash = ? AND rel_path = ?`, sum[:], filepath.ToSlash(rel))
	return err
}

// DeleteByRelPath removes a record (used when a catalogued file is gone).
func (d *DB) DeleteByRelPath(rel string) error {
	_, err := d.db.Exec(`DELETE FROM files WHERE rel_path = ?`, filepath.ToSlash(rel))
	return err
}

// StartRun records a new archiving run and returns its id.
func (d *DB) StartRun(sources []string) (int64, error) {
	res, err := d.db.Exec(`INSERT INTO runs(started_at, sources) VALUES (?, ?)`,
		time.Now().Unix(), strings.Join(sources, "\n"))
	if err != nil {
		return 0, fmt.Errorf("start run: %w", err)
	}
	return res.LastInsertId()
}

// FinishRun stores the final status and stats of a run.
func (d *DB) FinishRun(id int64, status, statsJSON string) error {
	_, err := d.db.Exec(`UPDATE runs SET finished_at = ?, status = ?, stats_json = ? WHERE id = ?`,
		time.Now().Unix(), status, statsJSON, id)
	return err
}

// JournalStart records that a file copy has begun.
func (d *DB) JournalStart(runID int64, src, tmp string) (int64, error) {
	res, err := d.db.Exec(`INSERT INTO journal(run_id, src_path, tmp_path, state, updated_at)
		VALUES (?, ?, ?, ?, ?)`, runID, src, tmp, StatePending, time.Now().Unix())
	if err != nil {
		return 0, fmt.Errorf("journal start: %w", err)
	}
	return res.LastInsertId()
}

// JournalUpdate moves a journal entry to a new state.
func (d *DB) JournalUpdate(id int64, state, dstRel string, sum hashing.Sum) error {
	var h any
	if !sum.IsZero() {
		h = sum[:]
	}
	_, err := d.db.Exec(`UPDATE journal SET state = ?, dst_rel_path = COALESCE(?, dst_rel_path),
		hash = COALESCE(?, hash), updated_at = ? WHERE id = ?`,
		state, nullIfEmpty(dstRel), h, time.Now().Unix(), id)
	return err
}

// FailStale marks unfinished journal entries and runs from previous sessions
// as failed and returns the number of affected journal entries. It also
// prunes finished journal entries, which are only needed for recovery.
func (d *DB) FailStale() (int64, error) {
	res, err := d.db.Exec(`UPDATE journal SET state = ?, updated_at = ? WHERE state IN (?, ?)`,
		StateFailed, time.Now().Unix(), StatePending, StateWritten)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	_, _ = d.db.Exec(`UPDATE runs SET status = 'interrupted' WHERE status = 'running'`)
	_, _ = d.db.Exec(`DELETE FROM journal WHERE state = ?`, StateDone)
	if n > 0 {
		d.log.Warn("unfinished operations from previous run marked failed", "count", n)
	}
	return n, nil
}

// RecoverWritten finishes journal entries that were published into the
// archive (state "written") but not yet catalogued when the app stopped:
// if the file exists it is added to the index so it is not copied again.
func (d *DB) RecoverWritten() (int, error) {
	rows, err := d.db.Query(`SELECT id, src_path, dst_rel_path, hash FROM journal
		WHERE state = ? AND dst_rel_path IS NOT NULL AND hash IS NOT NULL`, StateWritten)
	if err != nil {
		return 0, err
	}
	type entry struct {
		id       int64
		src, rel string
		hash     []byte
	}
	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.src, &e.rel, &e.hash); err == nil {
			entries = append(entries, e)
		}
	}
	rows.Close()

	recovered := 0
	for _, e := range entries {
		info, err := os.Stat(filepath.Join(d.root, filepath.FromSlash(e.rel)))
		if err != nil || len(e.hash) != hashing.Size {
			continue
		}
		var sum hashing.Sum
		copy(sum[:], e.hash)
		cat, year := categoryYearFromRel(e.rel)
		kind := "photo"
		if cat == layout.CatVideo {
			kind = "video"
		}
		rec := FileRecord{Hash: sum, Size: info.Size(), RelPath: e.rel, Kind: kind, Category: string(cat),
			Year: year, DateSource: "recovered", OrigName: filepath.Base(e.src), AddedAt: info.ModTime()}
		if err := d.InsertBatch([]FileRecord{rec}); err != nil {
			return recovered, err
		}
		_ = d.JournalUpdate(e.id, StateDone, "", hashing.Sum{})
		recovered++
	}
	if recovered > 0 {
		d.log.Warn("recovered files published before an interruption", "count", recovered)
	}
	return recovered, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return filepath.ToSlash(s)
}

// KindStats aggregates count and size.
type KindStats struct {
	Count int64 `json:"count"`
	Bytes int64 `json:"bytes"`
}

// Stats summarises the archive contents.
type Stats struct {
	Total      KindStats            `json:"total"`
	ByCategory map[string]KindStats `json:"byCategory"`
	Years      []int                `json:"years"`
}

// Stats returns archive statistics.
func (d *DB) Stats() (Stats, error) {
	st := Stats{ByCategory: map[string]KindStats{}}
	rows, err := d.db.Query(`SELECT category, COUNT(*), COALESCE(SUM(size),0) FROM files GROUP BY category`)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var c string
		var ks KindStats
		if err := rows.Scan(&c, &ks.Count, &ks.Bytes); err != nil {
			rows.Close()
			return st, err
		}
		st.ByCategory[c] = ks
		st.Total.Count += ks.Count
		st.Total.Bytes += ks.Bytes
	}
	rows.Close()

	yrows, err := d.db.Query(`SELECT DISTINCT year FROM files WHERE year > 0 ORDER BY year`)
	if err != nil {
		return st, err
	}
	defer yrows.Close()
	for yrows.Next() {
		var y int
		if err := yrows.Scan(&y); err == nil {
			st.Years = append(st.Years, y)
		}
	}
	return st, yrows.Err()
}
