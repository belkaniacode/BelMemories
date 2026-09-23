// Package logging configures the application-wide structured logger.
//
// Logs are written as JSON to a rotating file in the user config directory
// (<UserConfigDir>/MemoryArchive/logs/app.log). The level is controlled by the
// LOG_LEVEL environment variable (debug|info|warn|error, default info).
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// AppDirName is the per-user directory name for settings and logs.
const AppDirName = "MemoryArchive"

// Options controls logger setup.
type Options struct {
	// Dir overrides the log directory (defaults to <UserConfigDir>/MemoryArchive/logs).
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

// DefaultLogDir returns <UserConfigDir>/MemoryArchive/logs.
func DefaultLogDir() string {
	return filepath.Join(AppConfigDir(), "logs")
}

// AppConfigDir returns <UserConfigDir>/MemoryArchive (or a local fallback).
func AppConfigDir() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = "."
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
