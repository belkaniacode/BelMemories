// Package drives lists mounted disks the user can pick as source or target.
package drives

// Drive is a mounted volume.
type Drive struct {
	Path       string `json:"path"`
	Label      string `json:"label"`
	FreeBytes  uint64 `json:"freeBytes"`
	TotalBytes uint64 `json:"totalBytes"`
	Removable  bool   `json:"removable"`
}
