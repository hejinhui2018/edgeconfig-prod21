package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

type App struct {
	Stdout io.Writer
	Stderr io.Writer
	Logger *slog.Logger
}

func (a *App) Run(ctx context.Context, args []string) error {
	if a.Stdout == nil {
		a.Stdout = io.Discard
	}
	if a.Stderr == nil {
		a.Stderr = io.Discard
	}
	if a.Logger == nil {
		a.Logger = slog.Default()
	}
	if len(args) == 0 {
		return a.usageError("command is required")
	}
	switch args[0] {
	case "init":
		return a.runInit(ctx, args[1:])
	case "serve":
		return a.runServe(ctx, args[1:])
	case "rollout":
		return a.runRollout(ctx, args[1:])
	case "replay":
		return a.runReplay(ctx, args[1:])
	case "smoke":
		return a.runSmoke(ctx, args[1:])
	case "help", "-h", "--help":
		_, err := fmt.Fprint(a.Stdout, usage)
		return err
	default:
		return a.usageError("unknown command %q", args[0])
	}
}

func (a *App) usageError(format string, values ...any) error {
	_, _ = fmt.Fprint(a.Stderr, usage)
	return fmt.Errorf(format, values...)
}

const usage = `EdgeConfig configuration rollout service

Usage:
  edgeconfig init [-data-dir PATH]
  edgeconfig serve [-address HOST:PORT] [-data-dir PATH]
  edgeconfig rollout -id ID [-data-dir PATH]
  edgeconfig replay [-data-dir PATH]
  edgeconfig smoke
`
