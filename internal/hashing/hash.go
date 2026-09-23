// Package hashing computes BLAKE3 content hashes used for deduplication.
package hashing

import (
	"context"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"sync"
	"time"

	"github.com/zeebo/blake3"
	"golang.org/x/time/rate"

	"belmemories/internal/logging"
)

// Size is the digest length in bytes.
const Size = 32

// Sum is a BLAKE3-256 digest.
type Sum [Size]byte

// String returns the hex form.
func (s Sum) String() string { return hex.EncodeToString(s[:]) }

// IsZero reports an unset digest.
func (s Sum) IsZero() bool { return s == Sum{} }

// BufSize is the I/O buffer size used for hashing and copying.
const BufSize = 1 << 20

var bufPool = sync.Pool{New: func() any { b := make([]byte, BufSize); return &b }}

// GetBuf returns a pooled 1 MiB buffer; release it with PutBuf.
func GetBuf() *[]byte { return bufPool.Get().(*[]byte) }

// PutBuf returns a buffer to the pool.
func PutBuf(b *[]byte) { bufPool.Put(b) }

// New returns a fresh BLAKE3 hasher.
func New() hash.Hash { return blake3.New() }

// SumOf finalises a hasher into a Sum.
func SumOf(h hash.Hash) Sum {
	var s Sum
	copy(s[:], h.Sum(nil))
	return s
}

// Reader wraps r with cancellation checks and optional rate limiting.
type Reader struct {
	ctx     context.Context
	r       io.Reader
	limiter *rate.Limiter
}

// NewReader returns a context-aware, optionally throttled reader.
func NewReader(ctx context.Context, r io.Reader, limiter *rate.Limiter) *Reader {
	return &Reader{ctx: ctx, r: r, limiter: limiter}
}

func (cr *Reader) Read(p []byte) (int, error) {
	if err := cr.ctx.Err(); err != nil {
		return 0, err
	}
	if cr.limiter != nil && len(p) > cr.limiter.Burst() {
		p = p[:cr.limiter.Burst()]
	}
	n, err := cr.r.Read(p)
	if n > 0 && cr.limiter != nil {
		if werr := cr.limiter.WaitN(cr.ctx, n); werr != nil {
			return n, werr
		}
	}
	return n, err
}

// HashFile computes the BLAKE3 digest of a file.
func HashFile(ctx context.Context, path string, limiter *rate.Limiter) (Sum, error) {
	start := time.Now()
	f, err := os.Open(path)
	if err != nil {
		logging.Component("hashing").Error("open failed", "path", path, "err", err)
		return Sum{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	h := New()
	buf := GetBuf()
	defer PutBuf(buf)
	n, err := io.CopyBuffer(h, NewReader(ctx, f, limiter), *buf)
	if err != nil {
		if ctx.Err() == nil {
			logging.Component("hashing").Error("read failed", "path", path, "err", err)
		}
		return Sum{}, fmt.Errorf("hash %s: %w", path, err)
	}
	sum := SumOf(h)
	if n > 100<<20 {
		logging.Component("hashing").Debug("hashed large file", "path", path, "bytes", n, "took", time.Since(start).String())
	}
	return sum, nil
}

// TeeHasher hashes everything written through it to w.
type TeeHasher struct {
	w io.Writer
	h hash.Hash
	n int64
}

// NewTeeHasher wraps w so written bytes are also hashed.
func NewTeeHasher(w io.Writer) *TeeHasher { return &TeeHasher{w: w, h: New()} }

func (t *TeeHasher) Write(p []byte) (int, error) {
	n, err := t.w.Write(p)
	if n > 0 {
		t.h.Write(p[:n])
		t.n += int64(n)
	}
	return n, err
}

// Sum returns the digest of all bytes written so far.
func (t *TeeHasher) Sum() Sum { return SumOf(t.h) }

// Written returns the number of bytes written.
func (t *TeeHasher) Written() int64 { return t.n }

// NewLimiter returns a byte-rate limiter or nil when bytesPerSec <= 0.
func NewLimiter(bytesPerSec int64) *rate.Limiter {
	if bytesPerSec <= 0 {
		return nil
	}
	burst := BufSize
	if bytesPerSec < int64(burst) {
		burst = int(bytesPerSec)
	}
	return rate.NewLimiter(rate.Limit(bytesPerSec), burst)
}
