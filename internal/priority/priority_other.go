//go:build !linux && !windows

package priority

func setBackground(idleIO bool) error { return nil }

func restoreNormal() error { return nil }
