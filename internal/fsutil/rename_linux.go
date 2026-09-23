//go:build !windows

package fsutil

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"

	"belmemories/internal/logging"
)

// RenameNoReplace atomically renames oldPath to newPath, failing with
// ErrExists if newPath exists. Filesystems without renameat2 support fall
// back to link+unlink, and finally to check-then-rename (single writer).
func RenameNoReplace(oldPath, newPath string) error {
	err := unix.Renameat2(unix.AT_FDCWD, oldPath, unix.AT_FDCWD, newPath, unix.RENAME_NOREPLACE)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, unix.EEXIST):
		return ErrExists
	case errors.Is(err, unix.EINVAL), errors.Is(err, unix.ENOSYS), errors.Is(err, unix.ENOTSUP):
		// e.g. exFAT/NTFS via FUSE: try hard link which fails on existing target.
	default:
		return err
	}

	lerr := os.Link(oldPath, newPath)
	if lerr == nil {
		return os.Remove(oldPath)
	}
	if errors.Is(lerr, os.ErrExist) {
		return ErrExists
	}
	logging.Component("fsutil").Debug("hard link unsupported, using checked rename", "err", lerr)
	if _, serr := os.Lstat(newPath); serr == nil {
		return ErrExists
	} else if !errors.Is(serr, os.ErrNotExist) {
		return serr
	}
	return os.Rename(oldPath, newPath)
}

// syncDir fsyncs a directory so a rename survives power loss.
func syncDir(dir string) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = f.Sync()
	_ = f.Close()
}

func isFatalDiskErr(err error) bool {
	return errors.Is(err, unix.ENOSPC) || errors.Is(err, unix.EIO) ||
		errors.Is(err, unix.EROFS) || errors.Is(err, unix.EDQUOT)
}
