//go:build !linux

package main

import "errors"

// openWithActivation is Linux-only; other systems use their own launcher.
func openWithActivation(string) error { return errors.ErrUnsupported }
