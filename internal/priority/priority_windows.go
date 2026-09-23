//go:build windows

package priority

import "golang.org/x/sys/windows"

const (
	processModeBackgroundBegin = 0x00100000
	processModeBackgroundEnd   = 0x00200000
)

// setBackground enables background processing mode, which lowers CPU, I/O
// and memory priority of the process.
func setBackground(idleIO bool) error {
	h := windows.CurrentProcess()
	if err := windows.SetPriorityClass(h, processModeBackgroundBegin); err != nil {
		// Fallback when background mode is unavailable.
		return windows.SetPriorityClass(h, windows.BELOW_NORMAL_PRIORITY_CLASS)
	}
	return nil
}

func restoreNormal() error {
	h := windows.CurrentProcess()
	_ = windows.SetPriorityClass(h, processModeBackgroundEnd)
	return windows.SetPriorityClass(h, windows.NORMAL_PRIORITY_CLASS)
}
