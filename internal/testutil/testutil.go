// Package testutil builds small synthetic media files for tests: JPEGs with
// EXIF, PNGs with/without alpha and MP4 files with an mvhd creation time.
package testutil

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// Exif describes EXIF fields to embed.
type Exif struct {
	Make             string
	Model            string
	DateTimeOriginal string // "2006:01:02 15:04:05"
}

// JPEG returns a w×h JPEG with a gradient; seed changes pixel content so
// different seeds give different bytes.
func JPEG(t testing.TB, w, h int, seed uint8, exif *Exif) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x) + seed, uint8(y), seed * 3, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	if exif == nil {
		return data
	}
	app1 := buildAPP1(*exif)
	out := make([]byte, 0, len(data)+len(app1))
	out = append(out, data[:2]...) // SOI
	out = append(out, app1...)
	return append(out, data[2:]...)
}

// PNG returns a w×h PNG; alpha=true uses a transparent NRGBA image.
func PNG(t testing.TB, w, h int, seed uint8, alpha bool) []byte {
	t.Helper()
	var img image.Image
	if alpha {
		m := image.NewNRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				m.Set(x, y, color.NRGBA{seed, uint8(x), uint8(y), uint8(x % 256)})
			}
		}
		img = m
	} else {
		m := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				m.Set(x, y, color.RGBA{seed, uint8(x), uint8(y), 255})
			}
		}
		// Encode as opaque RGB (png encoder drops alpha when fully opaque? no:
		// convert to YCbCr-like Gray+RGB via a paletteless opaque image).
		img = opaque{m}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// opaque reports itself as Opaque so the PNG encoder writes RGB without alpha.
type opaque struct{ *image.RGBA }

func (o opaque) Opaque() bool { return true }

// MP4 returns a minimal ISO-BMFF file with ftyp + moov/mvhd(creation) + mdat
// payload (payload makes different files differ in content).
func MP4(created time.Time, payload []byte) []byte {
	var b bytes.Buffer
	// ftyp
	writeBox(&b, "ftyp", append([]byte("isom\x00\x00\x02\x00"), []byte("isomiso2mp41")...))

	epoch := time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
	secs := uint32(created.Sub(epoch) / time.Second)
	var mvhd bytes.Buffer
	mvhd.Write([]byte{0, 0, 0, 0}) // version 0 + flags
	be32(&mvhd, secs)              // creation
	be32(&mvhd, secs)              // modification
	be32(&mvhd, 1000)              // timescale
	be32(&mvhd, 0)                 // duration
	be32(&mvhd, 0x00010000)        // rate
	mvhd.Write([]byte{0x01, 0x00}) // volume
	mvhd.Write(make([]byte, 10))   // reserved
	for _, v := range []uint32{0x00010000, 0, 0, 0, 0x00010000, 0, 0, 0, 0x40000000} {
		be32(&mvhd, v)
	}
	mvhd.Write(make([]byte, 24)) // pre_defined
	be32(&mvhd, 2)               // next track id

	var moov bytes.Buffer
	writeBox(&moov, "mvhd", mvhd.Bytes())
	writeBox(&b, "moov", moov.Bytes())
	writeBox(&b, "mdat", payload)
	return b.Bytes()
}

// WriteFile writes data to dir/name (creating dirs), setting mtime if non-zero.
func WriteFile(t testing.TB, dir, name string, data []byte, mtime time.Time) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if !mtime.IsZero() {
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func writeBox(b *bytes.Buffer, typ string, payload []byte) {
	be32(b, uint32(8+len(payload)))
	b.WriteString(typ)
	b.Write(payload)
}

func be32(b *bytes.Buffer, v uint32) {
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], v)
	b.Write(tmp[:])
}

// ---- minimal little-endian TIFF/EXIF builder ----

type ifdEntry struct {
	tag   uint16
	typ   uint16 // 2 = ASCII, 4 = LONG
	count uint32
	data  []byte // for ASCII
	long  uint32 // for LONG
}

func ascii(tag uint16, s string) ifdEntry {
	d := append([]byte(s), 0)
	return ifdEntry{tag: tag, typ: 2, count: uint32(len(d)), data: d}
}

func buildAPP1(e Exif) []byte {
	var ifd0 []ifdEntry
	if e.Make != "" {
		ifd0 = append(ifd0, ascii(0x010F, e.Make))
	}
	if e.Model != "" {
		ifd0 = append(ifd0, ascii(0x0110, e.Model))
	}
	var exifIFD []ifdEntry
	if e.DateTimeOriginal != "" {
		exifIFD = append(exifIFD, ascii(0x9003, e.DateTimeOriginal))
	}
	hasExif := len(exifIFD) > 0
	if hasExif {
		ifd0 = append(ifd0, ifdEntry{tag: 0x8769, typ: 4, count: 1}) // pointer patched below
	}
	sort.Slice(ifd0, func(i, j int) bool { return ifd0[i].tag < ifd0[j].tag })

	ifdSize := func(n int) int { return 2 + n*12 + 4 }
	dataSize := func(es []ifdEntry) int {
		s := 0
		for _, en := range es {
			if len(en.data) > 4 {
				s += len(en.data)
			}
		}
		return s
	}
	off0 := 8
	exifOff := off0 + ifdSize(len(ifd0)) + dataSize(ifd0)
	for i := range ifd0 {
		if ifd0[i].tag == 0x8769 {
			ifd0[i].long = uint32(exifOff)
		}
	}

	var tiff bytes.Buffer
	tiff.WriteString("II")
	le16(&tiff, 42)
	le32(&tiff, uint32(off0))
	writeIFD(&tiff, ifd0, off0)
	if hasExif {
		writeIFD(&tiff, exifIFD, exifOff)
	}

	var app bytes.Buffer
	app.Write([]byte{0xFF, 0xE1})
	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	var l [2]byte
	binary.BigEndian.PutUint16(l[:], uint16(len(payload)+2))
	app.Write(l[:])
	app.Write(payload)
	return app.Bytes()
}

func writeIFD(b *bytes.Buffer, es []ifdEntry, start int) {
	dataOff := start + 2 + len(es)*12 + 4
	var extra bytes.Buffer
	le16(b, uint16(len(es)))
	for _, e := range es {
		le16(b, e.tag)
		le16(b, e.typ)
		le32(b, e.count)
		switch {
		case e.typ == 4:
			le32(b, e.long)
		case len(e.data) <= 4:
			v := make([]byte, 4)
			copy(v, e.data)
			b.Write(v)
		default:
			le32(b, uint32(dataOff+extra.Len()))
			extra.Write(e.data)
		}
	}
	le32(b, 0) // next IFD
	b.Write(extra.Bytes())
}

func le16(b *bytes.Buffer, v uint16) {
	var tmp [2]byte
	binary.LittleEndian.PutUint16(tmp[:], v)
	b.Write(tmp[:])
}

func le32(b *bytes.Buffer, v uint32) {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], v)
	b.Write(tmp[:])
}
