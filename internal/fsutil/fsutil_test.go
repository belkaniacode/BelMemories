package fsutil

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"memoryarchive/internal/hashing"
)

func TestRenameNoReplaceNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	os.WriteFile(src, []byte("new"), 0o644)
	os.WriteFile(dst, []byte("original"), 0o644)

	if err := RenameNoReplace(src, dst); !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "original" {
		t.Fatalf("destination was overwritten: %q", got)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source must remain after failed rename: %v", err)
	}

	free := filepath.Join(dir, "free")
	if err := RenameNoReplace(src, free); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source should be moved")
	}
}

func TestNameCandidate(t *testing.T) {
	if got := NameCandidate("IMG_1.jpg", 0); got != "IMG_1.jpg" {
		t.Fatal(got)
	}
	if got := NameCandidate("IMG_1.jpg", 2); got != "IMG_1_2.jpg" {
		t.Fatal(got)
	}
	if got := NameCandidate("noext", 1); got != "noext_1" {
		t.Fatal(got)
	}
}

func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"normal.jpg":       "normal.jpg",
		`a:b*c?.jpg`:       "a_b_c_.jpg",
		"CON.jpg":          "_CON.jpg",
		"nul":              "_nul",
		"trail. ":          "trail",
		"фото отпуска.jpg": "фото отпуска.jpg",
		"":                 "file",
	}
	for in, want := range cases {
		if got := SanitizeName(in); got != want {
			t.Errorf("SanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
	long := string(bytes.Repeat([]byte("я"), 300)) + ".jpg"
	if got := SanitizeName(long); len(got) > maxNameBytes || filepath.Ext(got) != ".jpg" {
		t.Errorf("long name not truncated correctly: %d bytes, ext %q", len(got), filepath.Ext(got))
	}
}

func TestSafeCopyRenamesOnConflictAndVerifies(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.jpg")
	os.WriteFile(src, []byte("content-B"), 0o644)
	dstDir := filepath.Join(dir, "archive", "Фото", "2020")
	os.MkdirAll(dstDir, 0o755)
	os.WriteFile(filepath.Join(dstDir, "photo.jpg"), []byte("content-A"), 0o644)
	tmp := filepath.Join(dir, "archive", ".memoryarchive", "tmp")

	res, err := SafeCopy(context.Background(), src, dstDir, "photo.jpg", CopyOptions{TmpDir: tmp, Verify: true})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(res.FinalPath) != "photo_1.jpg" {
		t.Fatalf("final = %s, want photo_1.jpg", res.FinalPath)
	}
	a, _ := os.ReadFile(filepath.Join(dstDir, "photo.jpg"))
	if string(a) != "content-A" {
		t.Fatal("existing file changed")
	}
	want, _ := hashing.HashFile(context.Background(), src, nil)
	if res.Hash != want {
		t.Fatal("hash mismatch")
	}
	assertNoParts(t, tmp)
}

func TestSafeCopyAcceptRejectLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.jpg")
	os.WriteFile(src, []byte("dup"), 0o644)
	dstDir := filepath.Join(dir, "out")
	tmp := filepath.Join(dir, "tmp")
	reject := errors.New("dup")

	_, err := SafeCopy(context.Background(), src, dstDir, "x.jpg", CopyOptions{
		TmpDir: tmp,
		Accept: func(hashing.Sum) error { return reject },
	})
	if !errors.Is(err, reject) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "x.jpg")); !os.IsNotExist(err) {
		t.Fatal("rejected file must not be published")
	}
	assertNoParts(t, tmp)
}

func TestSafeCopyCancelled(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	os.WriteFile(src, bytes.Repeat([]byte{1}, 3<<20), 0o644)
	tmp := filepath.Join(dir, "tmp")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SafeCopy(ctx, src, filepath.Join(dir, "out"), "a.bin", CopyOptions{TmpDir: tmp}); err == nil {
		t.Fatal("expected cancellation error")
	}
	assertNoParts(t, tmp)
	if _, err := os.Stat(filepath.Join(dir, "out", "a.bin")); !os.IsNotExist(err) {
		t.Fatal("cancelled copy must not publish")
	}
}

func TestSafeCopyExpectedHashMismatch(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	os.WriteFile(src, []byte("abc"), 0o644)
	var wrong hashing.Sum
	wrong[0] = 1
	_, err := SafeCopy(context.Background(), src, filepath.Join(dir, "out"), "a", CopyOptions{TmpDir: filepath.Join(dir, "tmp"), ExpectedHash: wrong})
	if !errors.Is(err, ErrVerify) {
		t.Fatalf("err = %v, want ErrVerify", err)
	}
}

func TestSourceReadErrorIsNotFatal(t *testing.T) {
	dir := t.TempDir()
	_, err := SafeCopy(context.Background(), filepath.Join(dir, "missing"), dir, "a", CopyOptions{TmpDir: filepath.Join(dir, "tmp")})
	if !errors.Is(err, ErrSourceRead) || errors.Is(err, ErrDiskFatal) {
		t.Fatalf("err = %v, want ErrSourceRead only", err)
	}
}

func TestCleanupTmp(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.part"), nil, 0o644)
	os.WriteFile(filepath.Join(dir, "keep.txt"), nil, 0o644)
	n, err := CleanupTmp(dir)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Fatal("non-part file removed")
	}
	if n, err := CleanupTmp(filepath.Join(dir, "missing")); err != nil || n != 0 {
		t.Fatal("missing dir must be ok")
	}
}

func assertNoParts(t *testing.T, tmp string) {
	t.Helper()
	entries, _ := os.ReadDir(tmp)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".part" {
			t.Fatalf("leftover part file: %s", e.Name())
		}
	}
}
