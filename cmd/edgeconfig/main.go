package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"edgeconfig/cli"
	"edgeconfig/config"
)

func main() {
	configuration, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	app := &cli.App{Stdout: os.Stdout, Stderr: os.Stderr, Logger: config.Logger(configuration.LogLevel)}
	if err := app.Run(ctx, os.Args[1:]); err != nil && !errors.Is(err, context.Canceled) {
		app.Logger.Error("command failed", "error", err)
		os.Exit(1)
	}
}
