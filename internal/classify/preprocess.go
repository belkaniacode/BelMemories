package classify

import (
	"errors"
	"fmt"
	"image"
	"os"

	"golang.org/x/image/draw"
)

// clipSize is the CLIP ViT-B/32 input resolution.
const clipSize = 224

// maxDecodePixels protects memory: bigger images are not decoded for CLIP.
const maxDecodePixels = 100_000_000

var (
	clipMean = [3]float32{0.48145466, 0.4578275, 0.40821073}
	clipStd  = [3]float32{0.26862954, 0.26130258, 0.27577711}
)

// errTooLarge means the image is too big to decode safely.
var errTooLarge = errors.New("image too large to decode")

// decodableExts are formats the Go standard/x decoders can decode fully.
var decodableExts = map[string]bool{
	"jpg": true, "jpeg": true, "jpe": true, "jfif": true, "png": true,
	"gif": true, "webp": true, "bmp": true, "tif": true, "tiff": true,
}

// CanDecode reports whether CLIP preprocessing supports the extension.
func CanDecode(ext string) bool { return decodableExts[ext] }

// preprocess decodes an image and returns a 1×3×224×224 float32 tensor
// (NCHW) with CLIP normalisation: center square crop, bilinear resize.
func preprocess(path string, hdr ImageHeader) ([]float32, error) {
	if hdr.OK && int64(hdr.Width)*int64(hdr.Height) > maxDecodePixels {
		return nil, errTooLarge
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	if side <= 0 {
		return nil, errors.New("empty image")
	}
	x0 := b.Min.X + (b.Dx()-side)/2
	y0 := b.Min.Y + (b.Dy()-side)/2
	crop := image.Rect(x0, y0, x0+side, y0+side)

	dst := image.NewRGBA(image.Rect(0, 0, clipSize, clipSize))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, crop, draw.Src, nil)

	const plane = clipSize * clipSize
	out := make([]float32, 3*plane)
	pix := dst.Pix
	for i := 0; i < plane; i++ {
		r := float32(pix[i*4]) / 255
		g := float32(pix[i*4+1]) / 255
		bl := float32(pix[i*4+2]) / 255
		// Transparent pixels were composited over black by draw.Src; that is
		// fine for classification.
		out[i] = (r - clipMean[0]) / clipStd[0]
		out[plane+i] = (g - clipMean[1]) / clipStd[1]
		out[2*plane+i] = (bl - clipMean[2]) / clipStd[2]
	}
	return out, nil
}
