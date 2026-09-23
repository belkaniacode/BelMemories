// Package fsutil implements crash-safe, never-overwriting file copies into the
// archive plus helpers for free space and file names.
//
// Invariants:
//   - the source is only read;
//   - data is written to a .part file, fsynced, optionally re-read and
//     verified, and only then atomically renamed into place;
//   - an existing destination file is never replaced.
package fsutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"memoryarchive/internal/hashing"
	"memoryarchive/internal/logging"
)

var (
	// ErrExists means the destination name is taken.
	ErrExists = errors.New("destination already exists")
	// ErrDiskFatal wraps write errors that make continuing unsafe (disk full,
	// I/O error, read-only filesystem). The whole run must stop.
	ErrDiskFatal = errors.New("fatal destination disk error")
	// ErrSourceRead wraps errors reading the source file (skip the file).
	ErrSourceRead = errors.New("source read error")
	// ErrVerify means the written data does not match what was read.
	ErrVerify = errors.New("verification failed")
)

// maxNameAttempts bounds name_N.ext probing.
const maxNameAttempts = 10000

// CopyOptions configures SafeCopy.
type CopyOptions struct {
	// TmpDir must be on the same filesystem as the destination directory.
	TmpDir string
	// Limiter throttles reading (nil = unlimited).
	Limiter *rate.Limiter
	// Verify re-reads the written file and compares hashes before publishing.
	Verify bool
	// ExpectedHash, when set, must equal the hash of the bytes copied
	// (detects the source changing between hashing and copying).
	ExpectedHash hashing.Sum
	// OnTmp is called with the .part path once it is created (for journaling).
	OnTmp func(tmpPath string)
	// Accept is called with the content hash before publishing. Returning an
	// error discards the copy (e.g. a duplicate detected only after hashing).
	Accept func(sum hashing.Sum) error
}

// CopyResult describes a completed copy.
type CopyResult struct {
	FinalPath string
	Hash      hashing.Sum
	Bytes     int64
}

// SafeCopy copies src into dstDir under name (or name_1, name_2… if taken).
func SafeCopy(ctx context.Context, src, dstDir, name string, opts CopyOptions) (CopyResult, error) {
	log := logging.Component("fsutil").With("src", src)
	var res CopyResult

	srcFile, err := os.Open(src)
	if err != nil {
		return res, fmt.Errorf("%w: open: %v", ErrSourceRead, err)
	}
	defer srcFile.Close()
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return res, fmt.Errorf("%w: stat: %v", ErrSourceRead, err)
	}

	if err := os.MkdirAll(opts.TmpDir, 0o755); err != nil {
		return res, classifyWriteErr(fmt.Errorf("create tmp dir: %w", err))
	}
	tmpPath := filepath.Join(opts.TmpDir, randomID()+".part")
	tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return res, classifyWriteErr(fmt.Errorf("create part file: %w", err))
	}
	log.Debug("part file created", "tmp", tmpPath)
	if opts.OnTmp != nil {
		opts.OnTmp(tmpPath)
	}

	published := false
	defer func() {
		if !published {
			_ = os.Remove(tmpPath)
		}
	}()

	sum, n, err := copyData(ctx, tmp, srcFile, opts.Limiter)
	if err != nil {
		tmp.Close()
		return res, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return res, classifyWriteErr(fmt.Errorf("fsync: %w", err))
	}
	if err := tmp.Close(); err != nil {
		return res, classifyWriteErr(fmt.Errorf("close: %w", err))
	}
	if n != srcInfo.Size() {
		log.Warn("source size changed during copy", "expected", srcInfo.Size(), "copied", n)
	}
	if !opts.ExpectedHash.IsZero() && sum != opts.ExpectedHash {
		return res, fmt.Errorf("%w: source changed while copying", ErrVerify)
	}
	if opts.Verify {
		got, err := hashing.HashFile(ctx, tmpPath, nil)
		if err != nil {
			if ctx.Err() != nil {
				return res, ctx.Err()
			}
			return res, classifyWriteErr(fmt.Errorf("re-read for verify: %w", err))
		}
		if got != sum {
			log.Error("verification mismatch", "tmp", tmpPath)
			return res, fmt.Errorf("%w: written data differs from source", ErrVerify)
		}
		log.Debug("verified", "tmp", tmpPath)
	}
	if opts.Accept != nil {
		if err := opts.Accept(sum); err != nil {
			log.Debug("copy rejected before publish", "reason", err)
			return CopyResult{Hash: sum, Bytes: n}, err
		}
	}
	_ = os.Chtimes(tmpPath, time.Now(), srcInfo.ModTime())

	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return res, classifyWriteErr(fmt.Errorf("create destination dir: %w", err))
	}
	name = SanitizeName(name)
	for attempt := 0; attempt < maxNameAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		candidate := filepath.Join(dstDir, NameCandidate(name, attempt))
		err := RenameNoReplace(tmpPath, candidate)
		if errors.Is(err, ErrExists) {
			continue
		}
		if err != nil {
			return res, classifyWriteErr(fmt.Errorf("publish: %w", err))
		}
		published = true
		syncDir(dstDir)
		log.Debug("published", "dst", candidate, "bytes", n)
		return CopyResult{FinalPath: candidate, Hash: sum, Bytes: n}, nil
	}
	return res, fmt.Errorf("no free name for %s in %s", name, dstDir)
}

// copyData streams src → dst while hashing. Source errors are wrapped with
// ErrSourceRead; destination errors are classified by classifyWriteErr.
func copyData(ctx context.Context, dst io.Writer, src io.Reader, limiter *rate.Limiter) (hashing.Sum, int64, error) {
	bufp := hashing.GetBuf()
	defer hashing.PutBuf(bufp)
	buf := *bufp
	if limiter != nil && limiter.Burst() < len(buf) {
		buf = buf[:limiter.Burst()]
	}
	tee := hashing.NewTeeHasher(dst)
	for {
		if err := ctx.Err(); err != nil {
			return hashing.Sum{}, tee.Written(), err
		}
		nr, rerr := src.Read(buf)
		if nr > 0 {
			if limiter != nil {
				if err := limiter.WaitN(ctx, nr); err != nil {
					return hashing.Sum{}, tee.Written(), err
				}
			}
			if _, werr := tee.Write(buf[:nr]); werr != nil {
				return hashing.Sum{}, tee.Written(), classifyWriteErr(fmt.Errorf("write: %w", werr))
			}
		}
		if rerr == io.EOF {
			return tee.Sum(), tee.Written(), nil
		}
		if rerr != nil {
			return hashing.Sum{}, tee.Written(), fmt.Errorf("%w: %v", ErrSourceRead, rerr)
		}
	}
}

// classifyWriteErr wraps fatal disk errors with ErrDiskFatal.
func classifyWriteErr(err error) error {
	if err == nil {
		return nil
	}
	if isFatalDiskErr(err) {
		logging.Component("fsutil").Error("fatal disk error", "err", err)
		return fmt.Errorf("%w: %v", ErrDiskFatal, err)
	}
	return err
}

// NameCandidate returns name for attempt 0 and "base_N.ext" afterwards.
func NameCandidate(name string, attempt int) string {
	if attempt == 0 {
		return name
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return fmt.Sprintf("%s_%d%s", base, attempt, ext)
}

// CleanupTmp removes leftover .part files from interrupted runs.
func CleanupTmp(tmpDir string) (int, error) {
	entries, err := os.ReadDir(tmpDir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".part") {
			continue
		}
		if err := os.Remove(filepath.Join(tmpDir, e.Name())); err == nil {
			removed++
		}
	}
	if removed > 0 {
		logging.Component("fsutil").Info("leftover part files removed", "dir", tmpDir, "count", removed)
	}
	return removed, nil
}

func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
