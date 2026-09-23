//go:build !linux && !windows

package systheme

func detect() (theme, source string) { return "", "" }
