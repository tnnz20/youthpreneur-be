package config

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// NewLogger creates a JSON logger writing to standard output with the requested minimum level.
func NewLogger(level string) *slog.Logger {
	return newLogger(os.Stdout, level)
}

func newLogger(w io.Writer, level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl}))
}
