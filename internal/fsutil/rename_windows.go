//go:build windows

package fsutil

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// RenameNoReplace moves oldPath to newPath without MOVEFILE_REPLACE_EXISTING,
// so an existing destination is never overwritten. (os.Rename on Windows
// replaces existing files and must not be used for publishing.)
func RenameNoReplace(oldPath, newPath string) error {
	from, err := windows.UTF16PtrFromString(longPath(oldPath))
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(longPath(newPath))
	if err != nil {
		return err
	}
	err = windows.MoveFileEx(from, to, windows.MOVEFILE_WRITE_THROUGH)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) || errors.Is(err, windows.ERROR_FILE_EXISTS) {
		return ErrExists
	}
	return err
}

// longPath adds the \\?\ prefix so paths longer than MAX_PATH work.
func longPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if strings.HasPrefix(abs, `\\?\`) {
		return abs
	}
	if strings.HasPrefix(abs, `\\`) {
		return `\\?\UNC\` + abs[2:]
	}
	return `\\?\` + abs
}

// syncDir is a no-op on Windows (MOVEFILE_WRITE_THROUGH flushes the move).
func syncDir(dir string) {}

func isFatalDiskErr(err error) bool {
	return errors.Is(err, windows.ERROR_DISK_FULL) ||
		errors.Is(err, windows.ERROR_HANDLE_DISK_FULL) ||
		errors.Is(err, windows.ERROR_WRITE_PROTECT) ||
		errors.Is(err, windows.ERROR_CRC) ||
		errors.Is(err, windows.ERROR_IO_DEVICE) ||
		errors.Is(err, windows.ERROR_DEVICE_NOT_CONNECTED) ||
		errors.Is(err, windows.ERROR_NOT_READY)
}
