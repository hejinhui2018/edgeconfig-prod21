package config

import (
	"log/slog"
	"os"
)

func Logger(level string) *slog.Logger {
	selected := slog.LevelInfo
	switch level {
	case "debug":
		selected = slog.LevelDebug
	case "warn":
		selected = slog.LevelWarn
	case "error":
		selected = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: selected}))
}
