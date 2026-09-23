// Package classify decides whether an image is a real-life photo (people,
// animals, places, everyday life) or a "picture" (logo, icon, screenshot,
// meme, document, illustration). Fast rules handle obvious cases; a CLIP
// model resolves the uncertain ones when available.
package classify

import (
	"image"
	"image/color"
	_ "image/gif"  // register decoder
	_ "image/jpeg" // register decoder
	_ "image/png"  // register decoder
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/bmp"  // register decoder
	_ "golang.org/x/image/tiff" // register decoder
	_ "golang.org/x/image/webp" // register decoder

	"memoryarchive/internal/layout"
	"memoryarchive/internal/logging"
	"memoryarchive/internal/media"
	"memoryarchive/internal/metadata"
)

// Decision is the outcome of the rule-based stage.
type Decision string

const (
	DecPhoto   Decision = "photo"
	DecPicture Decision = "picture"
	DecUnsure  Decision = "unsure"
)

// Verdict is the final classification of a file.
type Verdict struct {
	Category layout.Category `json:"category"`
	Method   string          `json:"method"` // rules | clip | default
	Reason   string          `json:"reason"`
	Label    string          `json:"label,omitempty"`
	Score    float64         `json:"score,omitempty"`
}

// ImageHeader holds facts read without decoding pixels.
type ImageHeader struct {
	Format   string
	Width    int
	Height   int
	HasAlpha bool
	OK       bool
}

// ReadHeader reads image dimensions/colour model via image.DecodeConfig.
func ReadHeader(path string) ImageHeader {
	f, err := os.Open(path)
	if err != nil {
		return ImageHeader{}
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return ImageHeader{}
	}
	return ImageHeader{
		Format:   format,
		Width:    cfg.Width,
		Height:   cfg.Height,
		HasAlpha: hasAlpha(cfg.ColorModel),
		OK:       true,
	}
}

func hasAlpha(m color.Model) bool {
	switch m {
	case color.NRGBAModel, color.NRGBA64Model, color.RGBAModel, color.RGBA64Model, color.AlphaModel, color.Alpha16Model:
		return true
	}
	if p, ok := m.(color.Palette); ok {
		for _, c := range p {
			if _, _, _, a := c.RGBA(); a < 0xffff {
				return true
			}
		}
	}
	return false
}

var screenshotNameHints = []string{
	"screenshot", "screen shot", "screen_shot", "снимок экрана", "скриншот",
	"scr_", "scrn", "capture", "снимок_экрана",
}

// Heuristic applies fast rules. It never reads pixel data.
func Heuristic(item media.Kind, path, ext string, info metadata.DateInfo, hdr ImageHeader) (Decision, string) {
	lowerName := strings.ToLower(filepath.Base(path))

	switch {
	case media.IsRaw(ext):
		return DecPhoto, "raw camera file"
	case ext == "heic" || ext == "heif":
		return DecPhoto, "heic from phone camera"
	case info.HasCameraInfo:
		return DecPhoto, "exif camera " + strings.TrimSpace(info.Make+" "+info.Model)
	case ext == "svg" || ext == "ico":
		return DecPicture, "vector/icon format"
	}
	for _, hint := range screenshotNameHints {
		if strings.Contains(lowerName, hint) {
			return DecPicture, "screenshot file name"
		}
	}

	w, h := hdr.Width, hdr.Height
	if w == 0 {
		w, h = info.Width, info.Height
	}
	if w > 0 && h > 0 && max(w, h) < 256 {
		return DecPicture, "small image"
	}
	if hdr.HasAlpha && (ext == "png" || ext == "gif" || ext == "webp") {
		return DecPicture, "transparent image"
	}
	if ext == "gif" {
		return DecPicture, "gif animation/graphics"
	}
	if ext == "png" && isScreenSize(w, h) {
		return DecPicture, "png with screen resolution"
	}
	if info.HasExif && info.Source == metadata.SourceExif {
		return DecPhoto, "exif capture date"
	}
	return DecUnsure, "no decisive signal"
}

// heuristicVerdict converts a rules decision into a Verdict (Unsure → Photo).
func heuristicVerdict(dec Decision, reason string) Verdict {
	switch dec {
	case DecPicture:
		return Verdict{Category: layout.CatPicture, Method: "rules", Reason: reason}
	case DecPhoto:
		return Verdict{Category: layout.CatPhoto, Method: "rules", Reason: reason}
	default:
		return Verdict{Category: layout.CatPhoto, Method: "default", Reason: reason}
	}
}

func logVerdict(path string, v Verdict) {
	logging.Component("classify").Debug("verdict", "path", path, "category", v.Category,
		"method", v.Method, "reason", v.Reason, "label", v.Label, "score", v.Score)
}
