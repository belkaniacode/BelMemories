package classify

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"belmemories/internal/layout"
	"belmemories/internal/media"
	"belmemories/internal/metadata"
	"belmemories/internal/scanner"
	"belmemories/internal/testutil"
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
	dir := FindModelDir(os.Getenv("BELMEMORIES_MODEL_DIR"))
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

// Scores below mirror real misclassifications: photos of people in a car or
// on a quad bike scored as "banner"/"screenshot" by CLIP.
func TestClipVerdictKeepsPeopleInPhotos(t *testing.T) {
	cases := []struct {
		name string
		res  ClipResult
		want layout.Category
	}{
		{"man driving, scored as screenshot", ClipResult{TopLabel: "screenshot", PictureProb: 0.78, HumanProb: 0.3}, layout.CatPhoto},
		{"no picture signal", ClipResult{TopLabel: "person", PictureProb: 0.1, HumanProb: 0.8}, layout.CatPhoto},
		{"app icon", ClipResult{TopLabel: "icon", PictureProb: 0.97, HumanProb: 0.01}, layout.CatPicture},
		{"meme with a face, overwhelming picture", ClipResult{TopLabel: "meme", PictureProb: 0.95, HumanProb: 0.4}, layout.CatPicture},
		{"illustration without people", ClipResult{TopLabel: "illustration", PictureProb: 0.7, HumanProb: 0.05}, layout.CatPicture},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if v := clipVerdict(tc.res, 0.5); v.Category != tc.want {
				t.Fatalf("category = %s (%s), want %s", v.Category, v.Reason, tc.want)
			}
		})
	}
}

func TestScreenshotWithPeopleStaysPhoto(t *testing.T) {
	if v := screenVerdict(ClipResult{TopLabel: "people", PictureProb: 0.05, HumanProb: 0.88}, "screenshot file name"); v.Category != layout.CatPhoto {
		t.Fatalf("screenshot of friends: category = %s, want photo", v.Category)
	}
	if v := screenVerdict(ClipResult{TopLabel: "screenshot", PictureProb: 0.9, HumanProb: 0.05}, "screenshot file name"); v.Category != layout.CatPicture {
		t.Fatalf("plain screenshot: category = %s, want picture", v.Category)
	}
}

// Only images that actually show through (a real transparent area) count as
// transparent. Regular photos saved as PNG — RGB, or RGBA with alpha 254–255
// everywhere (Stable Diffusion/ComfyUI output) — must not.
func TestTransparencyDetection(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, img image.Image) string {
		p := filepath.Join(dir, name)
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		return p
	}
	const w, h = 400, 300
	rgb := image.NewRGBA(image.Rect(0, 0, w, h)) // opaque → encoded as truecolour PNG
	nearOpaque := image.NewNRGBA(image.Rect(0, 0, w, h))
	logo := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.NRGBA{uint8(x), uint8(y), 120, 255}
			rgb.Set(x, y, c)
			c.A = 254 + uint8((x+y)%2)
			nearOpaque.SetNRGBA(x, y, c)
			if x > w/4 && x < 3*w/4 && y > h/4 && y < 3*h/4 {
				logo.SetNRGBA(x, y, color.NRGBA{200, 30, 30, 255})
			} // outside: fully transparent background
		}
	}
	cases := []struct {
		name string
		img  image.Image
		want bool
	}{
		{"photo-rgb.png", rgb, false},
		{"photo-alpha254.png", nearOpaque, false},
		{"logo-transparent.png", logo, true},
	}
	for _, c := range cases {
		if got := ReadHeader(write(c.name, c.img)).HasAlpha; got != c.want {
			t.Errorf("%s: HasAlpha = %v, want %v", c.name, got, c.want)
		}
	}
}

// Started from the desktop menu the working directory is $HOME and the binary
// sits elsewhere: the model installed in the per-user data dir must be found.
func TestFindModelDirInUserDataDir(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("LOCALAPPDATA", data)
	models := filepath.Join(data, "BelMemories", "models")
	if err := os.MkdirAll(models, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(models, ModelFile), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir()) // like $HOME: no models/ here
	if got := FindModelDir(""); got != models {
		t.Fatalf("FindModelDir = %q, want %q", got, models)
	}
}
