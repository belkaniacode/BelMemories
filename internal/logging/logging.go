// Package logging configures the application-wide structured logger.
//
// Logs are written as JSON to a rotating file in the user config directory
// (<UserConfigDir>/BelMemories/logs/app.log). The level is controlled by the
// LOG_LEVEL environment variable (debug|info|warn|error, default info).
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// AppDirName is the per-user directory name for settings and logs.
const AppDirName = "BelMemories"

// legacyAppDirName is the directory used before the rename to BelMemories.
const legacyAppDirName = "MemoryArchive"

// MigrateLegacyDir moves <UserConfigDir>/MemoryArchive to the current
// directory name once, so settings and logs survive the rename. It runs
// before logging is set up and therefore returns what happened instead of
// logging it.
func MigrateLegacyDir() (moved bool, err error) {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		return false, nil
	}
	oldDir, newDir := filepath.Join(base, legacyAppDirName), filepath.Join(base, AppDirName)
	if _, err := os.Stat(newDir); err == nil {
		return false, nil
	}
	if _, err := os.Stat(oldDir); err != nil {
		return false, nil
	}
	if err := os.Rename(oldDir, newDir); err != nil {
		return false, err
	}
	return true, nil
}

// Options controls logger setup.
type Options struct {
	// Dir overrides the log directory (defaults to <UserConfigDir>/BelMemories/logs).
	Dir string
	// Stderr duplicates log output to stderr (useful in dev mode).
	Stderr bool
}

// Setup installs a JSON slog logger as the default and returns it together
// with the log file path. If the log file cannot be opened it falls back to
// stderr only, so logging never blocks application startup.
func Setup(opts Options) (*slog.Logger, string) {
	level := ParseLevel(os.Getenv("LOG_LEVEL"))

	dir := opts.Dir
	if dir == "" {
		dir = DefaultLogDir()
	}

	var writers []io.Writer
	logPath := filepath.Join(dir, "app.log")
	if err := os.MkdirAll(dir, 0o755); err == nil {
		writers = append(writers, &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    10, // MB
			MaxBackups: 5,
		})
	} else {
		logPath = ""
	}
	if opts.Stderr || len(writers) == 0 {
		writers = append(writers, os.Stderr)
	}

	handler := slog.NewJSONHandler(io.MultiWriter(writers...), &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger, logPath
}

// DefaultLogDir returns <UserConfigDir>/BelMemories/logs.
func DefaultLogDir() string {
	return filepath.Join(AppConfigDir(), "logs")
}

// AppConfigDir returns <UserConfigDir>/BelMemories (or a local fallback).
func AppConfigDir() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = "."
	}
	return filepath.Join(base, AppDirName)
}

// AppDataDir is the per-user data folder for large files such as the CLIP
// model: $XDG_DATA_HOME (default ~/.local/share) on Linux, %LOCALAPPDATA% on
// Windows, ~/Library/Application Support on macOS.
func AppDataDir() string {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("LOCALAPPDATA")
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			base = filepath.Join(home, "Library", "Application Support")
		}
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			if home, err := os.UserHomeDir(); err == nil {
				base = filepath.Join(home, ".local", "share")
			}
		}
	}
	if base == "" {
		return AppConfigDir()
	}
	return filepath.Join(base, AppDirName)
}

// ParseLevel converts a LOG_LEVEL string into a slog level (default info).
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Component returns a child logger tagged with the given component name.
func Component(name string) *slog.Logger {
	return slog.Default().With("component", name)
}
