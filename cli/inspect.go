package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"time"

	"edgeconfig/config"
	"edgeconfig/domain"
	"edgeconfig/persistence"
	"edgeconfig/recovery"
)

func (a *App) runRollout(ctx context.Context, args []string) error {
	defaults := config.Default()
	flags := flag.NewFlagSet("rollout", flag.ContinueOnError)
	flags.SetOutput(a.Stderr)
	dataDir := flags.String("data-dir", defaults.DataDir, "persistent data directory")
	id := flags.String("id", "", "rollout id")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return fmt.Errorf("-id is required")
	}
	store, err := persistence.OpenFileStore(*dataDir)
	if err != nil {
		return err
	}
	engine, _, err := recovery.Recover(ctx, store, time.Now)
	if err != nil {
		return err
	}
	value, err := engine.GetRollout(domain.RolloutID(*id))
	if err != nil {
		return err
	}
	return json.NewEncoder(a.Stdout).Encode(value)
}

func (a *App) runReplay(ctx context.Context, args []string) error {
	defaults := config.Default()
	flags := flag.NewFlagSet("replay", flag.ContinueOnError)
	flags.SetOutput(a.Stderr)
	dataDir := flags.String("data-dir", defaults.DataDir, "persistent data directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	store, err := persistence.OpenFileStore(*dataDir)
	if err != nil {
		return err
	}
	verification, err := recovery.Verify(ctx, store)
	if err != nil {
		return err
	}
	_, report, err := recovery.Recover(ctx, store, time.Now)
	if err != nil {
		return err
	}
	return json.NewEncoder(a.Stdout).Encode(map[string]any{"verification": verification, "recovery": report})
}
