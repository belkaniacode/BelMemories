//go:build windows

package drives

import (
	"golang.org/x/sys/windows"

	"memoryarchive/internal/logging"
)

// List returns all ready fixed/removable drives (A:–Z:).
func List() []Drive {
	log := logging.Component("drives")
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		log.Warn("GetLogicalDrives failed", "err", err)
		return nil
	}
	var out []Drive
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		p, _ := windows.UTF16PtrFromString(root)
		typ := windows.GetDriveType(p)
		if typ != windows.DRIVE_FIXED && typ != windows.DRIVE_REMOVABLE {
			continue
		}
		var avail, total, free uint64
		if err := windows.GetDiskFreeSpaceEx(p, &avail, &total, &free); err != nil {
			continue // not ready (e.g. empty card reader)
		}
		label := root
		var name [windows.MAX_PATH + 1]uint16
		if err := windows.GetVolumeInformation(p, &name[0], uint32(len(name)), nil, nil, nil, nil, 0); err == nil {
			if v := windows.UTF16ToString(name[:]); v != "" {
				label = v + " (" + root[:2] + ")"
			}
		}
		out = append(out, Drive{Path: root, Label: label, FreeBytes: avail, TotalBytes: total,
			Removable: typ == windows.DRIVE_REMOVABLE})
	}
	log.Debug("drives listed", "count", len(out))
	return out
}
