//go:build !windows

package metadata

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
