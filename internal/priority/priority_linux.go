//go:build linux

package priority

import (
	"errors"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

const (
	ioprioWhoProcess = 1
	ioprioClassBE    = 2
	ioprioClassIdle  = 3
	ioprioClassShift = 13
)

func ioprioSet(tid, class, level int) error {
	prio := class<<ioprioClassShift | level
	_, _, errno := unix.Syscall(unix.SYS_IOPRIO_SET, ioprioWhoProcess, uintptr(tid), uintptr(prio))
	if errno != 0 {
		return errno
	}
	return nil
}

// threadIDs lists all threads of this process. On Linux nice and I/O priority
// are per-thread; threads created later inherit from their creator, so
// lowering every existing thread covers the whole Go runtime.
func threadIDs() []int {
	entries, err := os.ReadDir("/proc/self/task")
	if err != nil {
		return []int{0}
	}
	ids := make([]int, 0, len(entries))
	for _, e := range entries {
		if id, err := strconv.Atoi(e.Name()); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// setBackground sets nice 10 and best-effort level 7 (or idle) I/O priority.
func setBackground(idleIO bool) error {
	class, level := ioprioClassBE, 7
	if idleIO {
		class, level = ioprioClassIdle, 0
	}
	var firstErr error
	for _, tid := range threadIDs() {
		if err := unix.Setpriority(unix.PRIO_PROCESS, tid, 10); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := ioprioSet(tid, class, level); err != nil && firstErr == nil && !errors.Is(err, unix.ESRCH) {
			firstErr = err
		}
	}
	return firstErr
}

// restoreNormal resets I/O priority to the default best-effort level. Raising
// CPU priority back needs privileges, so niceness stays lowered.
func restoreNormal() error {
	for _, tid := range threadIDs() {
		_ = ioprioSet(tid, ioprioClassBE, 4)
	}
	return nil
}
