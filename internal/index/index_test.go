package index

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"belmemories/internal/hashing"
	"belmemories/internal/testutil"
)

func TestInsertAndLookup(t *testing.T) {
	root := t.TempDir()
	db, err := Open(root, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var h hashing.Sum
	h[0] = 7
	if err := db.InsertBatch([]FileRecord{{Hash: h, Size: 123, RelPath: "Фото/2020/a.jpg", Kind: "photo", Category: "photo", Year: 2020, DateSource: "exif", OrigName: "a.jpg"}}); err != nil {
		t.Fatal(err)
	}
	if ok, _ := db.HasSize(123); !ok {
		t.Fatal("size not found")
	}
	if ok, _ := db.HasSize(124); ok {
		t.Fatal("unexpected size")
	}
	rel, ok, err := db.LookupHash(h)
	if err != nil || !ok || rel != "Фото/2020/a.jpg" {
		t.Fatalf("lookup = %q %v %v", rel, ok, err)
	}
	// Duplicate hash is ignored, not an error.
	if err := db.InsertBatch([]FileRecord{{Hash: h, Size: 123, RelPath: "Фото/2020/b.jpg", Kind: "photo", Category: "photo"}}); err != nil {
		t.Fatal(err)
	}
	st, err := db.Stats()
	if err != nil || st.Total.Count != 1 || len(st.Years) != 1 {
		t.Fatalf("stats = %+v err=%v", st, err)
	}

	// Read-only open sees the same data.
	ro, err := Open(root, true)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if _, ok, _ := ro.LookupHash(h); !ok {
		t.Fatal("read-only lookup failed")
	}
}

func TestReadOnlyMissing(t *testing.T) {
	if _, err := Open(t.TempDir(), true); err != ErrNoIndex {
		t.Fatalf("err = %v, want ErrNoIndex", err)
	}
}

func TestJournalRecovery(t *testing.T) {
	root := t.TempDir()
	db, _ := Open(root, false)
	defer db.Close()

	p := testutil.WriteFile(t, root, "Видео/2019/v.mp4", []byte("video-bytes"), testutilZero)
	sum, _ := hashing.HashFile(context.Background(), p, nil)
	run, _ := db.StartRun([]string{"/src"})
	id, _ := db.JournalStart(run, "/src/v.mp4", "/tmp/x.part")
	if err := db.JournalUpdate(id, StateWritten, "Видео/2019/v.mp4", sum); err != nil {
		t.Fatal(err)
	}
	pending, _ := db.JournalStart(run, "/src/other.mp4", "/tmp/y.part")
	_ = pending

	n, err := db.RecoverWritten()
	if err != nil || n != 1 {
		t.Fatalf("recovered %d err=%v", n, err)
	}
	if rel, ok, _ := db.LookupHash(sum); !ok || rel != "Видео/2019/v.mp4" {
		t.Fatalf("recovered record missing: %q", rel)
	}
	failed, err := db.FailStale()
	if err != nil || failed != 1 {
		t.Fatalf("failed=%d err=%v", failed, err)
	}
}

func TestReindex(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFile(t, root, "Фото/2018/a.jpg", testutil.JPEG(t, 16, 16, 1, nil), testutilZero)
	testutil.WriteFile(t, root, "Без даты/Видео/b.mp4", []byte("not really mp4 but mp4 ext"), testutilZero)
	testutil.WriteFile(t, root, "Фото/2018/notes.txt", []byte("ignored"), testutilZero)

	db, _ := Open(root, false)
	defer db.Close()
	if !db.NeedsReindex() {
		t.Fatal("expected NeedsReindex")
	}
	p, err := db.Reindex(context.Background(), nil)
	if err != nil || p.Added != 2 {
		t.Fatalf("reindex added %d err=%v", p.Added, err)
	}
	if db.NeedsReindex() {
		t.Fatal("index should be populated")
	}
	if _, err := os.Stat(filepath.Join(root, ".memoryarchive", "index.db")); err != nil {
		t.Fatal(err)
	}
}
