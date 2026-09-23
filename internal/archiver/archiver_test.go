package archiver

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"memoryarchive/internal/classify"
	"memoryarchive/internal/config"
	"memoryarchive/internal/priority"
	"memoryarchive/internal/scanner"
	"memoryarchive/internal/testutil"
)

var old = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

// makeSource builds a messy source tree and returns its root.
func makeSource(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	cam := func(seed uint8, date string) []byte {
		return testutil.JPEG(t, 320, 240, seed, &testutil.Exif{Make: "Samsung", Model: "SM-G991B", DateTimeOriginal: date})
	}
	photo := cam(1, "2019:05:12 14:30:00")
	testutil.WriteFile(t, src, "phone/DCIM/a.jpg", photo, time.Time{})
	testutil.WriteFile(t, src, "backup/copy of a.jpg", photo, time.Time{}) // exact duplicate, other name
	testutil.WriteFile(t, src, "trip/same.jpg", cam(2, "2020:08:01 09:00:00"), time.Time{})
	testutil.WriteFile(t, src, "party/same.jpg", cam(3, "2020:12:31 23:00:00"), time.Time{}) // same name, other content
	testutil.WriteFile(t, src, "misc/nodate.jpg", testutil.JPEG(t, 300, 300, 4, nil), old)
	testutil.WriteFile(t, src, "video/clip.mp4", testutil.MP4(time.Date(2012, 6, 30, 0, 0, 0, 0, time.UTC), []byte("frames")), time.Time{})
	testutil.WriteFile(t, src, "web/icon.png", testutil.PNG(t, 48, 48, 5, false), time.Date(2021, 2, 2, 0, 0, 0, 0, time.UTC))
	testutil.WriteFile(t, src, "docs/notes.txt", []byte("not media"), time.Time{})
	return src
}

func scan(t *testing.T, roots ...string) []scanner.Item {
	t.Helper()
	res, err := scanner.Scan(context.Background(), scanner.Options{Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	return res.Items
}

func run(t *testing.T, ctx context.Context, items []scanner.Item, root string, dry bool, others []string) *Report {
	t.Helper()
	r := NewRunner(classify.New(classify.Options{ModelDir: filepath.Join(t.TempDir(), "no-model")}))
	rep, err := r.Run(ctx, Plan{
		Items: items, Sources: []string{"src"}, ArchiveRoot: root,
		Options: Options{Profile: priority.ProfileFor(config.LoadHigh, 0), Verify: true, DryRun: dry,
			OtherArchives: others, ReportDir: t.TempDir()},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

// archiveFiles lists archive-relative paths of content files (no .memoryarchive).
func archiveFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == ".memoryarchive" {
			return fs.SkipDir
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func TestArchiveEndToEnd(t *testing.T) {
	src := makeSource(t)
	items := scan(t, src)
	if len(items) != 7 {
		t.Fatalf("scanned %d media files, want 7", len(items))
	}
	root := t.TempDir()

	rep := run(t, context.Background(), items, root, false, nil)
	if rep.Status != StatusCompleted {
		t.Fatalf("status %s: %s", rep.Status, rep.Message)
	}
	got := archiveFiles(t, root)
	want := []string{
		"Без даты/Фото/nodate.jpg",
		"Видео/2012/clip.mp4",
		"Картинки/2021/icon.png",
		"Фото/2019/a.jpg",
		"Фото/2020/same.jpg",
		"Фото/2020/same_1.jpg",
	}
	// a.jpg vs "copy of a.jpg": whichever wins the race is kept, the other is a duplicate.
	for i, g := range got {
		if g == "Фото/2019/copy of a.jpg" {
			got[i] = "Фото/2019/a.jpg"
		}
	}
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("archive layout:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	s := rep.Stats
	if s.Copied != 6 || s.Duplicates != 1 || s.Renamed != 1 || s.Errors != 0 || s.NoDate != 1 {
		t.Fatalf("stats: %+v", s)
	}
	if s.Videos != 1 || s.Pictures != 1 || s.Photos != 4 {
		t.Fatalf("category stats: %+v", s)
	}
	if rep.CSVPath == "" || rep.ReportPath == "" {
		t.Fatal("reports not written")
	}
	assertNoParts(t, root)

	// Second run: everything is already archived.
	rep2 := run(t, context.Background(), scan(t, src), root, false, nil)
	if rep2.Stats.Copied != 0 || rep2.Stats.Duplicates != 7 {
		t.Fatalf("second run stats: %+v", rep2.Stats)
	}
	if n := len(archiveFiles(t, root)); n != 6 {
		t.Fatalf("second run changed archive: %d files", n)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	src := makeSource(t)
	root := t.TempDir()
	rep := run(t, context.Background(), scan(t, src), root, true, nil)
	if rep.Status != StatusCompleted || rep.Stats.Copied == 0 || len(rep.Planned) == 0 {
		t.Fatalf("dry run: %+v", rep.Stats)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatalf("dry run wrote into destination: %v", entries)
	}
}

func TestCancelledRunLeavesArchiveConsistent(t *testing.T) {
	src := makeSource(t)
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rep := run(t, ctx, scan(t, src), root, false, nil)
	if rep.Status != StatusCancelled {
		t.Fatalf("status = %s", rep.Status)
	}
	assertNoParts(t, root)

	// A normal run afterwards completes the archive.
	rep2 := run(t, context.Background(), scan(t, src), root, false, nil)
	if rep2.Status != StatusCompleted || len(archiveFiles(t, root)) != 6 {
		t.Fatalf("after cancel: %s, files %v", rep2.Status, archiveFiles(t, root))
	}
}

func TestOtherArchiveDedup(t *testing.T) {
	src := makeSource(t)
	first := t.TempDir()
	run(t, context.Background(), scan(t, src), first, false, nil)

	second := t.TempDir()
	rep := run(t, context.Background(), scan(t, src), second, false, []string{first, filepath.Join(t.TempDir(), "unplugged")})
	if rep.Stats.Copied != 0 || rep.Stats.Duplicates != 7 {
		t.Fatalf("cross-archive dedup failed: %+v", rep.Stats)
	}
	if len(rep.MissingArchives) != 1 {
		t.Fatalf("missing archives = %v", rep.MissingArchives)
	}
}

func TestPauseResume(t *testing.T) {
	src := makeSource(t)
	root := t.TempDir()
	r := NewRunner(nil)
	r.Pause()
	done := make(chan *Report)
	go func() {
		rep, _ := r.Run(context.Background(), Plan{Items: scan(t, src), ArchiveRoot: root,
			Options: Options{Profile: priority.ProfileFor(config.LoadMedium, 0)}}, nil)
		done <- rep
	}()
	select {
	case <-done:
		t.Fatal("run finished while paused")
	case <-time.After(300 * time.Millisecond):
	}
	r.Resume()
	select {
	case rep := <-done:
		if rep.Status != StatusCompleted || rep.Stats.Copied != 6 {
			t.Fatalf("after resume: %s %+v", rep.Status, rep.Stats)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run did not finish after resume")
	}
}

func assertNoParts(t *testing.T, root string) {
	t.Helper()
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".part") {
			t.Fatalf("leftover part file %s", p)
		}
		return nil
	})
}
