//go:build linux

package drives

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/unix"

	"belmemories/internal/i18n"
	"belmemories/internal/logging"
)

// Pseudo and system filesystems that are never user data disks.
var skipFS = map[string]bool{
	"proc": true, "sysfs": true, "devtmpfs": true, "devpts": true, "tmpfs": true,
	"cgroup": true, "cgroup2": true, "pstore": true, "bpf": true, "tracefs": true,
	"debugfs": true, "securityfs": true, "configfs": true, "fusectl": true,
	"mqueue": true, "hugetlbfs": true, "autofs": true, "efivarfs": true,
	"binfmt_misc": true, "ramfs": true, "squashfs": true, "overlay": true,
	"nsfs": true, "rpc_pipefs": true, "fuse.portal": true, "fuse.gvfsd-fuse": true,
}

// List returns the home directory plus mounted data volumes (external disks
// under /run/media, /media, /mnt and other non-system mounts).
func List() []Drive {
	log := logging.Component("drives")
	var out []Drive
	seen := map[string]bool{}

	if home, err := os.UserHomeDir(); err == nil {
		if d, ok := stat(home, i18n.Pick("Домашняя папка", "Home folder"), false); ok {
			out = append(out, d)
			seen[home] = true
		}
	}

	f, err := os.Open("/proc/self/mounts")
	if err != nil {
		log.Warn("cannot read mounts", "err", err)
		return out
	}
	defer f.Close()

	var mounts []Drive
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		mnt, fstype := unescape(fields[1]), fields[2]
		if skipFS[fstype] || seen[mnt] {
			continue
		}
		removable := strings.HasPrefix(mnt, "/run/media/") || strings.HasPrefix(mnt, "/media/")
		userMount := removable || strings.HasPrefix(mnt, "/mnt/") || mnt == "/mnt" || mnt == "/"
		if !userMount && !strings.HasPrefix(mnt, "/home") && !strings.HasPrefix(mnt, "/data") {
			continue
		}
		if strings.HasPrefix(mnt, "/boot") {
			continue
		}
		label := filepath.Base(mnt)
		if mnt == "/" {
			label = i18n.Pick("Системный диск (/)", "System disk (/)")
		}
		if d, ok := stat(mnt, label, removable); ok {
			mounts = append(mounts, d)
			seen[mnt] = true
		}
	}
	sort.Slice(mounts, func(i, j int) bool {
		if mounts[i].Removable != mounts[j].Removable {
			return mounts[i].Removable
		}
		return mounts[i].Path < mounts[j].Path
	})
	out = append(out, mounts...)
	log.Debug("drives listed", "count", len(out))
	return out
}

func stat(path, label string, removable bool) (Drive, bool) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return Drive{}, false
	}
	return Drive{
		Path:       path,
		Label:      label,
		FreeBytes:  st.Bavail * uint64(st.Bsize),
		TotalBytes: st.Blocks * uint64(st.Bsize),
		Removable:  removable,
	}, true
}

// unescape decodes octal escapes (\040 = space) used in /proc/mounts.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			v := (int(s[i+1]-'0') << 6) | (int(s[i+2]-'0') << 3) | int(s[i+3]-'0')
			if v >= 0 && v < 256 {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
