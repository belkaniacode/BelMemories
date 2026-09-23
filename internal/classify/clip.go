package classify

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	ort "github.com/yalue/onnxruntime_go"

	"belmemories/internal/logging"
)

// Model file names inside the model directory.
const (
	ModelFile  = "clip-image.onnx"
	LabelsFile = "clip-labels.json"
)

// ONNX input/output names produced by tools/clip-export/export.py.
const (
	clipInput  = "pixel_values"
	clipOutput = "image_embeds"
)

// Label is one text prompt group with its precomputed CLIP text embedding.
type Label struct {
	Label     string    `json:"label"`
	Group     string    `json:"group"` // "photo" | "picture"
	Human     bool      `json:"human"` // the label describes people
	Embedding []float32 `json:"embedding"`
}

type labelsFile struct {
	Model      string  `json:"model"`
	Dim        int     `json:"dim"`
	LogitScale float64 `json:"logitScale"`
	Labels     []Label `json:"labels"`
}

// ClipResult is a CLIP classification.
type ClipResult struct {
	Group       string  // photo | picture
	PictureProb float64 // summed probability of picture labels
	HumanProb   float64 // summed probability of labels with people
	TopLabel    string
	TopProb     float64
}

// Clip wraps an ONNX Runtime session of the CLIP image encoder.
type Clip struct {
	mu         sync.Mutex
	session    *ort.DynamicAdvancedSession
	labels     []Label
	dim        int
	logitScale float32
}

var (
	envOnce sync.Once
	envErr  error
)

// LoadClip initialises ONNX Runtime and loads the model from modelDir.
// threads limits intra-op parallelism (keeps CPU load predictable).
func LoadClip(libPath, modelDir string, threads int) (*Clip, error) {
	log := logging.Component("classify")
	start := time.Now()

	modelPath := filepath.Join(modelDir, ModelFile)
	labelsPath := filepath.Join(modelDir, LabelsFile)
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("model not found: %s", modelPath)
	}
	lf, err := readLabels(labelsPath)
	if err != nil {
		return nil, err
	}

	envOnce.Do(func() {
		ort.SetSharedLibraryPath(libPath)
		envErr = ort.InitializeEnvironment()
	})
	if envErr != nil {
		return nil, fmt.Errorf("onnxruntime init (%s): %w", libPath, envErr)
	}

	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("session options: %w", err)
	}
	defer opts.Destroy()
	if threads < 1 {
		threads = 1
	}
	_ = opts.SetIntraOpNumThreads(threads)
	_ = opts.SetInterOpNumThreads(1)

	sess, err := ort.NewDynamicAdvancedSession(modelPath, []string{clipInput}, []string{clipOutput}, opts)
	if err != nil {
		return nil, fmt.Errorf("load model: %w", err)
	}
	scale := float32(lf.LogitScale)
	if scale <= 0 {
		scale = 100
	}
	log.Info("clip model loaded", "model", modelPath, "labels", len(lf.Labels), "threads", threads,
		"ortVersion", ort.GetVersion(), "took", time.Since(start).String())
	return &Clip{session: sess, labels: lf.Labels, dim: lf.Dim, logitScale: scale}, nil
}

func readLabels(path string) (*labelsFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("labels not found: %w", err)
	}
	var lf labelsFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parse labels: %w", err)
	}
	if len(lf.Labels) == 0 || lf.Dim <= 0 {
		return nil, errors.New("labels file is empty")
	}
	for i := range lf.Labels {
		if len(lf.Labels[i].Embedding) != lf.Dim {
			return nil, fmt.Errorf("label %q has embedding size %d, want %d",
				lf.Labels[i].Label, len(lf.Labels[i].Embedding), lf.Dim)
		}
		normalize(lf.Labels[i].Embedding)
	}
	return &lf, nil
}

// Classify runs the image encoder and compares with label embeddings.
func (c *Clip) Classify(path string, hdr ImageHeader) (ClipResult, error) {
	pixels, err := preprocess(path, hdr)
	if err != nil {
		return ClipResult{}, err
	}
	input, err := ort.NewTensor(ort.NewShape(1, 3, clipSize, clipSize), pixels)
	if err != nil {
		return ClipResult{}, fmt.Errorf("input tensor: %w", err)
	}
	defer input.Destroy()
	output, err := ort.NewEmptyTensor[float32](ort.NewShape(1, int64(c.dim)))
	if err != nil {
		return ClipResult{}, fmt.Errorf("output tensor: %w", err)
	}
	defer output.Destroy()

	c.mu.Lock()
	err = c.session.Run([]ort.Value{input}, []ort.Value{output})
	c.mu.Unlock()
	if err != nil {
		return ClipResult{}, fmt.Errorf("inference: %w", err)
	}

	emb := append([]float32(nil), output.GetData()...)
	normalize(emb)
	return c.score(emb), nil
}

func (c *Clip) score(emb []float32) ClipResult {
	logits := make([]float64, len(c.labels))
	maxLogit := math.Inf(-1)
	for i, l := range c.labels {
		var dot float32
		for j := range emb {
			dot += emb[j] * l.Embedding[j]
		}
		logits[i] = float64(dot * c.logitScale)
		maxLogit = math.Max(maxLogit, logits[i])
	}
	var sum float64
	for i := range logits {
		logits[i] = math.Exp(logits[i] - maxLogit)
		sum += logits[i]
	}
	var res ClipResult
	for i, l := range c.labels {
		p := logits[i] / sum
		if l.Group == "picture" {
			res.PictureProb += p
		}
		if l.Human {
			res.HumanProb += p
		}
		if p > res.TopProb {
			res.TopProb, res.TopLabel = p, l.Label
		}
	}
	res.Group = "photo"
	if res.PictureProb > 0.5 {
		res.Group = "picture"
	}
	return res
}

// Close releases the session.
func (c *Clip) Close() {
	if c == nil || c.session == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.session.Destroy()
	c.session = nil
}

func normalize(v []float32) {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	if s == 0 {
		return
	}
	inv := float32(1 / math.Sqrt(s))
	for i := range v {
		v[i] *= inv
	}
}

// FindSharedLibrary searches for the ONNX Runtime library next to the
// executable, in <modelDir>/lib, and in standard system locations.
func FindSharedLibrary(modelDir string) string {
	var names []string
	var sysDirs []string
	switch runtime.GOOS {
	case "windows":
		names = []string{"onnxruntime.dll"}
	case "darwin":
		names = []string{"libonnxruntime.dylib"}
	default:
		names = []string{"libonnxruntime.so", "onnxruntime.so"}
		sysDirs = []string{"/usr/lib", "/usr/local/lib", "/usr/lib64", "/usr/lib/x86_64-linux-gnu"}
	}

	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}
	if modelDir != "" {
		dirs = append(dirs, filepath.Join(modelDir, "lib", runtime.GOOS), filepath.Join(modelDir, "lib"), modelDir)
	}
	dirs = append(dirs, sysDirs...)

	for _, d := range dirs {
		for _, n := range names {
			p := filepath.Join(d, n)
			if _, err := os.Stat(p); err == nil {
				return p
			}
			// Versioned sonames: libonnxruntime.so.1.29.1
			if runtime.GOOS == "linux" {
				if m, _ := filepath.Glob(p + ".*"); len(m) > 0 {
					return m[len(m)-1]
				}
			}
		}
	}
	return ""
}

// FindModelDir returns the first directory containing the CLIP model files.
func FindModelDir(preferred string) string {
	var dirs []string
	if preferred != "" {
		dirs = append(dirs, preferred)
	}
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(exe), "models"))
	}
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, filepath.Join(wd, "models"))
	}
	dirs = append(dirs, filepath.Join(logging.AppConfigDir(), "models"))
	for _, d := range dirs {
		if _, err := os.Stat(filepath.Join(d, ModelFile)); err == nil {
			return d
		}
	}
	return ""
}
