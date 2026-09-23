package hashing

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestHashFileMatchesTee(t *testing.T) {
	data := bytes.Repeat([]byte("memoryarchive"), 300000) // > 1 MiB buffer
	p := filepath.Join(t.TempDir(), "f")
	os.WriteFile(p, data, 0o644)

	got, err := HashFile(context.Background(), p, nil)
	if err != nil {
		t.Fatal(err)
	}
	var sink bytes.Buffer
	tee := NewTeeHasher(&sink)
	tee.Write(data)
	if tee.Sum() != got || tee.Written() != int64(len(data)) {
		t.Fatal("tee hash differs from file hash")
	}

	limited, err := HashFile(context.Background(), p, NewLimiter(1<<30))
	if err != nil || limited != got {
		t.Fatal("limited hash differs")
	}
}

func TestHashFileCancelled(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	os.WriteFile(p, []byte("x"), 0o644)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := HashFile(ctx, p, nil); err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestDifferentContentDifferentHash(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	os.WriteFile(a, []byte("one"), 0o644)
	os.WriteFile(b, []byte("two"), 0o644)
	ha, _ := HashFile(context.Background(), a, nil)
	hb, _ := HashFile(context.Background(), b, nil)
	if ha == hb || ha.IsZero() {
		t.Fatal("hashes must differ and be non-zero")
	}
}
