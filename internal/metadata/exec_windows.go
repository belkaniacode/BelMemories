//go:build windows

package metadata

import (
	"os/exec"
	"syscall"
)

// hideWindow prevents a console window from flashing for each ffprobe call.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
