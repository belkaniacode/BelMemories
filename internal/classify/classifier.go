package classify

import (
	"errors"

	"belmemories/internal/i18n"
	"belmemories/internal/layout"
	"belmemories/internal/logging"
	"belmemories/internal/media"
	"belmemories/internal/metadata"
	"belmemories/internal/scanner"
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
		c.status = Status{Reason: i18n.Pick("модель CLIP не найдена (папка models/)", "CLIP model not found (models/ folder)")}
		log.Warn("clip model not found, rules-only mode")
		return c
	}
	lib := FindSharedLibrary(modelDir)
	c.status.ModelDir, c.status.LibPath = modelDir, lib
	if lib == "" {
		c.status.Reason = i18n.Pick("библиотека onnxruntime не найдена", "onnxruntime library not found")
		log.Warn("onnxruntime library not found, rules-only mode", "modelDir", modelDir)
		return c
	}
	clip, err := LoadClip(lib, modelDir, opts.Threads)
	if err != nil {
		c.status.Reason = i18n.Pick("ошибка загрузки нейросети: ", "neural network failed to load: ") + err.Error()
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

// People on an image keep it in Фото: CLIP often scores amateur/low-quality
// photos of people (in a car, on a quad bike, social-network reposts) as
// "banner" or "screenshot", and users expect those next to their photos.
const (
	humanMinProb    = 0.25 // people are likely present
	humanMaxPicture = 0.9  // ...unless CLIP is almost sure it is a picture
	screenHumanProb = 0.5  // a screenshot by name/size that mostly shows people
)

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
	useClip := c.clip != nil && CanDecode(item.Ext) && (dec == DecUnsure || dec == DecLikelyPicture)
	if !useClip {
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
		lvl("clip failed, using rules", "path", item.Path, "err", err)
		v := heuristicVerdict(dec, reason)
		if dec == DecUnsure {
			v.Reason = "clip error"
		}
		logVerdict(item.Path, v)
		return v
	}
	var v Verdict
	if dec == DecLikelyPicture {
		v = screenVerdict(res, reason)
	} else {
		v = clipVerdict(res, c.threshold)
	}
	logVerdict(item.Path, v)
	return v
}

// clipVerdict maps a CLIP result of an image without decisive rules.
func clipVerdict(res ClipResult, threshold float64) Verdict {
	v := Verdict{Category: layout.CatPhoto, Method: "clip", Label: res.TopLabel, Score: res.PictureProb,
		Reason: "clip: " + res.TopLabel}
	if res.PictureProb < threshold {
		return v
	}
	if res.HumanProb >= humanMinProb && res.PictureProb < humanMaxPicture {
		logging.Component("classify").Debug("[FIX] people override: kept in photos",
			"top", res.TopLabel, "picture", res.PictureProb, "human", res.HumanProb)
		v.Reason = "clip: people on image"
		return v
	}
	v.Category = layout.CatPicture
	return v
}

// screenVerdict decides a screenshot-like file: a picture unless it shows people.
func screenVerdict(res ClipResult, ruleReason string) Verdict {
	if res.HumanProb >= screenHumanProb {
		logging.Component("classify").Debug("[FIX] screenshot with people: kept in photos",
			"rule", ruleReason, "top", res.TopLabel, "human", res.HumanProb)
		return Verdict{Category: layout.CatPhoto, Method: "clip", Label: res.TopLabel, Score: res.PictureProb,
			Reason: "clip: people on " + ruleReason}
	}
	return Verdict{Category: layout.CatPicture, Method: "rules", Label: res.TopLabel, Score: res.PictureProb,
		Reason: ruleReason}
}
