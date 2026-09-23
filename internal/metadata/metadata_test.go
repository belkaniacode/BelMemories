package metadata

import (
	"testing"
	"time"

	"belmemories/internal/media"
	"belmemories/internal/scanner"
	"belmemories/internal/testutil"
)

func TestFromFilename(t *testing.T) {
	cases := []struct {
		name string
		want string // "2006-01-02" or "" for no match
	}{
		{"IMG_20190512_143000.jpg", "2019-05-12"},
		{"VID_20200101_000001.mp4", "2020-01-01"},
		{"PXL_20210314_101010123.jpg", "2021-03-14"},
		{"IMG-20180704-WA0001.jpg", "2018-07-04"},
		{"VID-20170102-WA0012.mp4", "2017-01-02"},
		{"Screenshot_2019-05-12-14-30-00.png", "2019-05-12"},
		{"photo_2022-11-05_18-20-33.jpg", "2022-11-05"},
		{"signal-2023-02-28-101010.jpg", "2023-02-28"},
		{"20160815_120000.jpg", "2016-08-15"},
		{"DSC_20150101.jpg", "2015-01-01"},
		{"1557664200000.jpg", "2019-05-12"},
		{"IMG_1234.JPG", ""},
		{"holiday.jpg", ""},
		{"IMG_20190231_120000.jpg", ""}, // Feb 31 is invalid
		{"IMG_20991231_120000.jpg", ""}, // future
		{"scan_17000101.jpg", ""},       // before 1980
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := FromFilename(c.name)
			if c.want == "" {
				if ok {
					t.Fatalf("expected no date, got %v", got)
				}
				return
			}
			if !ok {
				t.Fatalf("expected %s, got none", c.want)
			}
			if got.Format("2006-01-02") != c.want {
				t.Fatalf("got %s, want %s", got.Format("2006-01-02"), c.want)
			}
		})
	}
}

func TestExtractExif(t *testing.T) {
	dir := t.TempDir()
	data := testutil.JPEG(t, 64, 48, 1, &testutil.Exif{Make: "Canon", Model: "EOS 80D", DateTimeOriginal: "2014:07:21 10:11:12"})
	mtime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	p := testutil.WriteFile(t, dir, "a.jpg", data, mtime)

	info := Extract(scanner.Item{Path: p, Kind: media.Photo, Ext: "jpg", ModTime: mtime, Size: int64(len(data))})
	if info.Source != SourceExif {
		t.Fatalf("source = %s, want exif", info.Source)
	}
	if info.Year() != 2014 {
		t.Fatalf("year = %d, want 2014", info.Year())
	}
	if !info.HasCameraInfo || info.Make != "Canon" {
		t.Fatalf("camera info not detected: %+v", info)
	}
}

func TestExtractVideoContainer(t *testing.T) {
	dir := t.TempDir()
	created := time.Date(2012, 6, 30, 12, 0, 0, 0, time.UTC)
	data := testutil.MP4(created, []byte("payload"))
	mtime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	p := testutil.WriteFile(t, dir, "clip.mp4", data, mtime)

	info := Extract(scanner.Item{Path: p, Kind: media.Video, Ext: "mp4", ModTime: mtime, Size: int64(len(data))})
	if info.Source != SourceContainer || info.Year() != 2012 {
		t.Fatalf("got %s %d, want container 2012", info.Source, info.Year())
	}
}

func TestExtractFallbacks(t *testing.T) {
	dir := t.TempDir()
	data := testutil.JPEG(t, 32, 32, 2, nil)
	mtime := time.Date(2011, 3, 3, 0, 0, 0, 0, time.UTC)

	named := testutil.WriteFile(t, dir, "IMG-20100101-WA0001.jpg", data, mtime)
	info := Extract(scanner.Item{Path: named, Kind: media.Photo, Ext: "jpg", ModTime: mtime})
	if info.Source != SourceFilename || info.Year() != 2010 {
		t.Fatalf("filename fallback: got %s %d", info.Source, info.Year())
	}

	plain := testutil.WriteFile(t, dir, "plain.jpg", data, mtime)
	info = Extract(scanner.Item{Path: plain, Kind: media.Photo, Ext: "jpg", ModTime: mtime})
	if info.Source != SourceMtime || info.Year() != 2011 {
		t.Fatalf("mtime fallback: got %s %d", info.Source, info.Year())
	}

	info = Extract(scanner.Item{Path: plain, Kind: media.Photo, Ext: "jpg", ModTime: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)})
	if info.Source != SourceNone || info.Year() != 0 {
		t.Fatalf("no date: got %s %d", info.Source, info.Year())
	}
}
