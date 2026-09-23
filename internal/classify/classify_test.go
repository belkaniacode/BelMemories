package classify

import (
	"os"
	"testing"
	"time"

	"memoryarchive/internal/layout"
	"memoryarchive/internal/media"
	"memoryarchive/internal/metadata"
	"memoryarchive/internal/scanner"
	"memoryarchive/internal/testutil"
)

func classifyFile(t *testing.T, c *Classifier, name string, data []byte, kind media.Kind) Verdict {
	t.Helper()
	p := testutil.WriteFile(t, t.TempDir(), name, data, time.Time{})
	it := scanner.Item{Path: p, Kind: kind, Ext: media.Ext(p), Size: int64(len(data)), ModTime: time.Now()}
	return c.Classify(it, metadata.Extract(it))
}

func rulesOnly() *Classifier { return &Classifier{threshold: 0.5} }

func TestHeuristics(t *testing.T) {
	c := rulesOnly()
	cases := []struct {
		name string
		data []byte
		kind media.Kind
		want layout.Category
	}{
		{"camera.jpg", testutil.JPEG(t, 400, 300, 1, &testutil.Exif{Make: "Apple", Model: "iPhone 12", DateTimeOriginal: "2021:05:05 10:00:00"}), media.Photo, layout.CatPhoto},
		{"icon.png", testutil.PNG(t, 64, 64, 1, false), media.Photo, layout.CatPicture},
		{"logo.png", testutil.PNG(t, 600, 300, 2, true), media.Photo, layout.CatPicture},
		{"shot.png", testutil.PNG(t, 1080, 2340, 3, false), media.Photo, layout.CatPicture},
		{"Screenshot_2020-01-01.jpg", testutil.JPEG(t, 500, 500, 4, nil), media.Photo, layout.CatPicture},
		{"vector.svg", []byte("<svg xmlns='http://www.w3.org/2000/svg'/>"), media.Photo, layout.CatPicture},
		{"clip.mp4", testutil.MP4(time.Now().Add(-time.Hour), []byte("x")), media.Video, layout.CatVideo},
		// No decisive signal and no CLIP → Photo (safe default for personal pictures).
		{"whatsapp.jpg", testutil.JPEG(t, 800, 600, 5, nil), media.Photo, layout.CatPhoto},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := classifyFile(t, c, tc.name, tc.data, tc.kind)
			if v.Category != tc.want {
				t.Fatalf("category = %s (%s: %s), want %s", v.Category, v.Method, v.Reason, tc.want)
			}
		})
	}
}

func TestScreenSize(t *testing.T) {
	if !isScreenSize(1080, 1920) || !isScreenSize(1920, 1080) || isScreenSize(1600, 1200) {
		t.Fatal("screen size table broken")
	}
}

// TestClipModel runs only when the exported model and onnxruntime exist.
func TestClipModel(t *testing.T) {
	dir := FindModelDir(os.Getenv("MEMORYARCHIVE_MODEL_DIR"))
	if dir == "" || FindSharedLibrary(dir) == "" {
		t.Skip("CLIP model or onnxruntime not installed")
	}
	c := New(Options{ModelDir: dir, Threads: 2})
	defer c.Close()
	if !c.Status().ClipAvailable {
		t.Fatalf("clip not available: %s", c.Status().Reason)
	}
	v := classifyFile(t, c, "plain.jpg", testutil.JPEG(t, 300, 300, 9, nil), media.Photo)
	if v.Method != "clip" {
		t.Fatalf("expected clip method, got %s (%s)", v.Method, v.Reason)
	}
}
