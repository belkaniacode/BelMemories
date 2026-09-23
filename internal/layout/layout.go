// Package layout defines the on-disk structure of an archive.
package layout

import "path/filepath"

// Category is where a file is placed inside the archive.
type Category string

const (
	CatPhoto   Category = "photo"
	CatVideo   Category = "video"
	CatPicture Category = "picture"
)

// Top-level folder names.
const (
	DirPhotos   = "Фото"
	DirVideos   = "Видео"
	DirPictures = "Картинки"
	DirNoDate   = "Без даты"
	MetaDir     = ".memoryarchive"
)

// CategoryDir returns the top-level folder name for a category.
func CategoryDir(c Category) string {
	switch c {
	case CatVideo:
		return DirVideos
	case CatPicture:
		return DirPictures
	default:
		return DirPhotos
	}
}

// TargetDir returns the archive-relative directory for a file.
// year <= 0 means "no date" → Без даты/<category>.
func TargetDir(c Category, year int) string {
	if year <= 0 {
		return filepath.Join(DirNoDate, CategoryDir(c))
	}
	return filepath.Join(CategoryDir(c), itoa(year))
}

// ContentDirs lists top-level folders holding archived files.
func ContentDirs() []string {
	return []string{DirPhotos, DirVideos, DirPictures, DirNoDate}
}

// MetaPath joins parts under <root>/.memoryarchive.
func MetaPath(root string, parts ...string) string {
	return filepath.Join(append([]string{root, MetaDir}, parts...)...)
}

func itoa(n int) string {
	var b [8]byte
	i := len(b)
	for n > 0 && i > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
