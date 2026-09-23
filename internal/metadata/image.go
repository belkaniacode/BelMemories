package metadata

import (
	"os"
	"strings"

	"github.com/evanoberholster/imagemeta"

	"memoryarchive/internal/logging"
)

// Formats imagemeta can read EXIF from.
var exifExts = map[string]bool{
	"jpg": true, "jpeg": true, "jpe": true, "jfif": true, "tif": true, "tiff": true,
	"heic": true, "heif": true, "avif": true, "png": true, "webp": false,
	"cr2": true, "cr3": true, "crw": true, "nef": true, "nrw": true, "arw": true,
	"srf": true, "sr2": true, "dng": true, "orf": true, "rw2": true, "raf": true,
	"srw": true, "pef": true,
}

// fromImage reads EXIF. Source is SourceExif when a valid date was found,
// otherwise empty (camera facts may still be filled).
func fromImage(path, ext string) (info DateInfo) {
	if !exifExts[ext] {
		return info
	}
	log := logging.Component("metadata")

	f, err := os.Open(path)
	if err != nil {
		log.Warn("cannot open image for exif", "path", path, "err", err)
		return info
	}
	defer f.Close()

	// imagemeta can panic on malformed input; never let one file kill the run.
	defer func() {
		if r := recover(); r != nil {
			log.Warn("exif parser panic", "path", path, "panic", r)
		}
	}()

	ex, err := imagemeta.Decode(f)
	if err != nil {
		log.Debug("no exif", "path", path, "err", err)
		// Partial data may still be useful, fall through.
	}

	info.Make = strings.TrimSpace(ex.IFD0.Make)
	info.Model = strings.TrimSpace(ex.IFD0.Model)
	info.Software = strings.TrimSpace(ex.IFD0.Software)
	info.HasCameraInfo = info.Make != "" || info.Model != ""
	info.Width = int(ex.ExifIFD.PixelXDimension)
	info.Height = int(ex.ExifIFD.PixelYDimension)
	if info.Width == 0 {
		info.Width, info.Height = int(ex.IFD0.ImageWidth), int(ex.IFD0.ImageHeight)
	}

	if d := ex.OriginalDate(); validDate(d) {
		info.Time, info.Source = d, SourceExif
	} else if d := ex.DigitizedDate(); validDate(d) {
		info.Time, info.Source = d, SourceExif
	} else if d := ex.ModifyDate(); validDate(d) && info.HasCameraInfo {
		// IFD0 ModifyDate is only trusted for camera files; editors rewrite it.
		info.Time, info.Source = d, SourceExif
	}
	info.HasExif = info.HasCameraInfo || info.Source == SourceExif
	return info
}
