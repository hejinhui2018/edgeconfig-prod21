package cli

import (
	"context"
	"flag"
	"fmt"
	"time"

	"edgeconfig/config"
	"edgeconfig/persistence"
)

func (a *App) runInit(ctx context.Context, args []string) error {
	defaults := config.Default()
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(a.Stderr)
	dataDir := flags.String("data-dir", defaults.DataDir, "persistent data directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	store, err := persistence.OpenFileStore(*dataDir)
	if err != nil {
		return err
	}
	snapshot := persistence.EmptySnapshot()
	snapshot.CreatedAt = time.Now().UTC()
	if err := store.Save(ctx, snapshot); err != nil {
		return err
	}
	_, err = fmt.Fprintf(a.Stdout, "initialized EdgeConfig data at %s\n", *dataDir)
	return err
}
