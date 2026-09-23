package archiver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"

	"golang.org/x/time/rate"

	"memoryarchive/internal/fsutil"
	"memoryarchive/internal/hashing"
	"memoryarchive/internal/index"
	"memoryarchive/internal/layout"
	"memoryarchive/internal/media"
	"memoryarchive/internal/metadata"
	"memoryarchive/internal/scanner"
)

// errDuplicate rejects a copy whose content is already archived.
var errDuplicate = errors.New("duplicate content")

type pipeline struct {
	r       *Runner
	set     *archiveSet
	opts    Options
	root    string
	tmpDir  string
	runID   int64
	tr      *tracker
	coll    *collector
	log     *slog.Logger
	cancel  context.CancelCauseFunc
	limiter *rate.Limiter

	// seen maps hashes accepted in this run → archive path (writer-only).
	seen map[hashing.Sum]string
	// sizes accepted in this run (guarded by sizesMu; read by dedup workers).
	sizesMu sync.Mutex
	sizes   map[int64]bool
}

const chanBuf = 64

// run wires analyze → dedup → write and blocks until all stages finish.
func (p *pipeline) run(ctx context.Context, items []scanner.Item) {
	p.sizes = map[int64]bool{}
	src := make(chan scanner.Item, chanBuf)
	analyzed := make(chan *job, chanBuf)
	deduped := make(chan *job, chanBuf)

	go func() {
		defer close(src)
		for _, it := range items {
			select {
			case src <- it:
			case <-ctx.Done():
				return
			}
		}
	}()

	var wgA sync.WaitGroup
	for i := 0; i < defaultWorkers(p.opts.Profile.CPUWorkers); i++ {
		wgA.Add(1)
		go func() {
			defer wgA.Done()
			for it := range src {
				if !p.r.waitIfPaused(ctx) {
					return
				}
				j := p.analyze(it)
				select {
				case analyzed <- j:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() { wgA.Wait(); close(analyzed) }()

	var wgD sync.WaitGroup
	for i := 0; i < max(1, p.opts.Profile.Readers); i++ {
		wgD.Add(1)
		go func() {
			defer wgD.Done()
			for j := range analyzed {
				if !p.r.waitIfPaused(ctx) {
					return
				}
				if !p.dedup(ctx, j) {
					continue
				}
				select {
				case deduped <- j:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() { wgD.Wait(); close(deduped) }()

	// Single writer: sequential writes are gentlest for the destination disk.
	for j := range deduped {
		if !p.r.waitIfPaused(ctx) {
			break
		}
		p.write(ctx, j)
	}
	// Drain so upstream goroutines can exit after cancellation.
	for range deduped {
	}
}

// analyze determines date and category.
func (p *pipeline) analyze(it scanner.Item) *job {
	p.tr.c.current.Store(it.Path)
	info := metadata.Extract(it)
	j := &job{item: it, dateSrc: string(info.Source), year: info.Year()}
	if p.r.classifier != nil {
		v := p.r.classifier.Classify(it, info)
		j.category, j.method = v.Category, v.Method
	} else if it.Kind == media.Video {
		j.category, j.method = layout.CatVideo, "rules"
	} else {
		j.category, j.method = layout.CatPhoto, "default"
	}
	return j
}

// dedup pre-hashes files whose size already exists and drops duplicates.
// Returns false when the job is finished (duplicate or error).
func (p *pipeline) dedup(ctx context.Context, j *job) bool {
	size := j.item.Size
	known, err := p.set.hasSize(size)
	if err != nil {
		p.fail(j, fmt.Errorf("index: %w", err))
		return false
	}
	p.sizesMu.Lock()
	seenSize := p.sizes[size]
	p.sizes[size] = true
	p.sizesMu.Unlock()
	if !known && !seenSize {
		return true // unique size → hash while copying (single read)
	}

	sum, err := hashing.HashFile(ctx, j.item.Path, p.limiter)
	if err != nil {
		if ctx.Err() == nil {
			p.fail(j, err)
		}
		return false
	}
	j.hash = sum
	if where, ok, err := p.set.lookup(sum); err != nil {
		p.fail(j, fmt.Errorf("index: %w", err))
		return false
	} else if ok {
		p.duplicate(j, where)
		return false
	}
	return true
}

// write copies one file (or records the plan in dry-run mode).
func (p *pipeline) write(ctx context.Context, j *job) {
	it := j.item
	p.tr.c.current.Store(it.Path)
	if !j.hash.IsZero() {
		if where, ok := p.seen[j.hash]; ok {
			p.duplicate(j, where)
			return
		}
	}

	relDir := layout.TargetDir(j.category, j.year)
	dstDir := filepath.Join(p.root, relDir)
	name := fsutil.SanitizeName(filepath.Base(it.Path))

	if p.opts.DryRun {
		if !j.hash.IsZero() {
			p.seen[j.hash] = filepath.Join(dstDir, name)
		}
		p.done(j, FileResult{Src: it.Path, Dst: filepath.Join(dstDir, name), Action: ActPlanned}, false)
		return
	}

	free, err := fsutil.FreeSpace(p.root)
	if err == nil && int64(free) < it.Size+reserveBytes {
		p.log.Error("destination almost full, stopping", "free", free, "need", it.Size+reserveBytes)
		p.cancel(fmt.Errorf("%w: свободно %d МБ", ErrNoSpace, free>>20))
		return
	}

	var journalID int64
	res, err := fsutil.SafeCopy(ctx, it.Path, dstDir, name, fsutil.CopyOptions{
		TmpDir:       p.tmpDir,
		Limiter:      p.limiter,
		Verify:       p.opts.Verify,
		ExpectedHash: j.hash,
		OnTmp: func(tmp string) {
			if id, jerr := p.set.primary.JournalStart(p.runID, it.Path, tmp); jerr == nil {
				journalID = id
			}
		},
		Accept: func(sum hashing.Sum) error {
			if where, ok := p.seen[sum]; ok {
				return fmt.Errorf("%w: %s", errDuplicate, where)
			}
			if where, ok, lerr := p.set.lookup(sum); lerr != nil {
				return lerr
			} else if ok {
				return fmt.Errorf("%w: %s", errDuplicate, where)
			}
			return nil
		},
	})
	switch {
	case errors.Is(err, errDuplicate):
		p.journal(journalID, index.StateDone, "", res.Hash)
		p.duplicate(j, err.Error())
		return
	case err != nil:
		p.journal(journalID, index.StateFailed, "", hashing.Sum{})
		if ctx.Err() != nil {
			return // cancelled: the part file was removed, nothing to report
		}
		p.fail(j, err)
		if errors.Is(err, fsutil.ErrDiskFatal) {
			p.cancel(err)
		}
		return
	}

	rel, _ := filepath.Rel(p.root, res.FinalPath)
	p.journal(journalID, index.StateWritten, rel, res.Hash)
	p.seen[res.Hash] = res.FinalPath
	rec := index.FileRecord{
		Hash: res.Hash, Size: res.Bytes, RelPath: rel, Kind: string(it.Kind),
		Category: string(j.category), Year: j.year, DateSource: j.dateSrc,
		OrigName: filepath.Base(it.Path),
	}
	if err := p.set.primary.InsertBatch([]index.FileRecord{rec}); err != nil {
		// The file is safely in the archive; a later run will recover it from
		// the journal ("written" state) or via reindex.
		p.log.Error("index insert failed", "dst", res.FinalPath, "err", err)
	} else {
		p.journal(journalID, index.StateDone, "", hashing.Sum{})
	}
	p.tr.c.writtenBytes.Add(res.Bytes)
	renamed := filepath.Base(res.FinalPath) != name
	p.done(j, FileResult{Src: it.Path, Dst: res.FinalPath, Action: ActCopied, Renamed: renamed}, renamed)
	p.log.Debug("copied", "src", it.Path, "dst", res.FinalPath, "category", j.category,
		"year", j.year, "dateSource", j.dateSrc, "renamed", renamed)
}

func (p *pipeline) journal(id int64, state, rel string, sum hashing.Sum) {
	if id == 0 {
		return
	}
	if err := p.set.primary.JournalUpdate(id, state, rel, sum); err != nil {
		p.log.Warn("journal update failed", "id", id, "err", err)
	}
}

func (p *pipeline) done(j *job, r FileResult, renamed bool) {
	r.Category, r.Year, r.DateSource, r.Method = string(j.category), j.year, j.dateSrc, j.method
	p.tr.c.copied.Add(1)
	if renamed {
		p.tr.c.renamed.Add(1)
	}
	if j.year == 0 {
		p.tr.c.noDate.Add(1)
	}
	switch j.category {
	case layout.CatVideo:
		p.tr.c.videos.Add(1)
	case layout.CatPicture:
		p.tr.c.pictures.Add(1)
	default:
		p.tr.c.photos.Add(1)
	}
	p.finish(j)
	p.coll.add(r)
}

func (p *pipeline) duplicate(j *job, where string) {
	p.tr.c.duplicates.Add(1)
	p.finish(j)
	p.coll.add(FileResult{Src: j.item.Path, Action: ActDuplicate, DupOf: where,
		Category: string(j.category), Year: j.year, DateSource: j.dateSrc})
	p.log.Debug("duplicate skipped", "src", j.item.Path, "of", where)
}

func (p *pipeline) fail(j *job, err error) {
	p.tr.c.errors.Add(1)
	p.finish(j)
	p.coll.add(FileResult{Src: j.item.Path, Action: ActError, Err: err.Error()})
	p.log.Warn("file failed", "src", j.item.Path, "err", err)
}

func (p *pipeline) finish(j *job) {
	n := p.tr.c.processed.Add(1)
	p.tr.c.processedBytes.Add(j.item.Size)
	if n%1000 == 0 {
		p.log.Info("progress", "processed", n, "of", p.tr.total)
	}
}
