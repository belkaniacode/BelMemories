//go:build !windows

package fsutil

import "golang.org/x/sys/unix"

// FreeSpace returns bytes available to the current user on path's filesystem.
func FreeSpace(path string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, err
	}
	return st.Bavail * uint64(st.Bsize), nil
}
