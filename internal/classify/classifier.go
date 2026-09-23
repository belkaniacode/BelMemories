package classify

import (
	"errors"

	"memoryarchive/internal/layout"
	"memoryarchive/internal/logging"
	"memoryarchive/internal/media"
	"memoryarchive/internal/metadata"
	"memoryarchive/internal/scanner"
)

// Status describes classifier availability for the UI.
type Status struct {
	ClipAvailable bool   `json:"clipAvailable"`
	Reason        string `json:"reason"`
	ModelDir      string `json:"modelDir"`
	LibPath       string `json:"libPath"`
}

// Options configure the classifier.
type Options struct {
	ModelDir  string  // preferred model dir ("" = auto)
	Threads   int     // ONNX intra-op threads
	Threshold float64 // minimal picture probability to move to Картинки
}

// Classifier combines rules with an optional CLIP model.
type Classifier struct {
	clip      *Clip
	status    Status
	threshold float64
}

// New builds a classifier. A missing model or library is not an error: the
// classifier then works in rules-only mode and reports why in Status().
func New(opts Options) *Classifier {
	log := logging.Component("classify")
	c := &Classifier{threshold: opts.Threshold}
	if c.threshold <= 0 || c.threshold >= 1 {
		c.threshold = 0.5
	}

	modelDir := FindModelDir(opts.ModelDir)
	if modelDir == "" {
		c.status = Status{Reason: "модель CLIP не найдена (папка models/)"}
		log.Warn("clip model not found, rules-only mode")
		return c
	}
	lib := FindSharedLibrary(modelDir)
	c.status.ModelDir, c.status.LibPath = modelDir, lib
	if lib == "" {
		c.status.Reason = "библиотека onnxruntime не найдена"
		log.Warn("onnxruntime library not found, rules-only mode", "modelDir", modelDir)
		return c
	}
	clip, err := LoadClip(lib, modelDir, opts.Threads)
	if err != nil {
		c.status.Reason = "ошибка загрузки нейросети: " + err.Error()
		log.Warn("clip unavailable, rules-only mode", "err", err)
		return c
	}
	c.clip = clip
	c.status.ClipAvailable = true
	return c
}

// Status reports whether CLIP is active.
func (c *Classifier) Status() Status { return c.status }

// Close frees model resources.
func (c *Classifier) Close() {
	if c != nil && c.clip != nil {
		c.clip.Close()
	}
}

// Classify decides the archive category of a scanned item.
func (c *Classifier) Classify(item scanner.Item, info metadata.DateInfo) Verdict {
	if item.Kind == media.Video {
		return Verdict{Category: layout.CatVideo, Method: "rules", Reason: "video"}
	}

	var hdr ImageHeader
	if CanDecode(item.Ext) {
		hdr = ReadHeader(item.Path)
	}
	dec, reason := Heuristic(item.Kind, item.Path, item.Ext, info, hdr)
	if dec != DecUnsure || c.clip == nil || !CanDecode(item.Ext) {
		v := heuristicVerdict(dec, reason)
		logVerdict(item.Path, v)
		return v
	}

	res, err := c.clip.Classify(item.Path, hdr)
	if err != nil {
		lvl := logging.Component("classify").Error
		if errors.Is(err, errTooLarge) {
			lvl = logging.Component("classify").Debug
		}
		lvl("clip failed, defaulting to photo", "path", item.Path, "err", err)
		v := Verdict{Category: layout.CatPhoto, Method: "default", Reason: "clip error"}
		logVerdict(item.Path, v)
		return v
	}
	v := Verdict{Category: layout.CatPhoto, Method: "clip", Label: res.TopLabel, Score: res.PictureProb,
		Reason: "clip: " + res.TopLabel}
	if res.PictureProb >= c.threshold {
		v.Category = layout.CatPicture
	}
	logVerdict(item.Path, v)
	return v
}
