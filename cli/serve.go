package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"

	"edgeconfig/agent"
	"edgeconfig/api"
	"edgeconfig/config"
	"edgeconfig/persistence"
	"edgeconfig/recovery"
	"edgeconfig/rollout"
)

func (a *App) runServe(ctx context.Context, args []string) error {
	defaults, err := config.Load()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(a.Stderr)
	address := flags.String("address", defaults.Address, "HTTP listen address")
	dataDir := flags.String("data-dir", defaults.DataDir, "persistent data directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	store, err := persistence.OpenFileStore(*dataDir)
	if err != nil {
		return err
	}
	engine, report, err := recovery.Recover(ctx, store, time.Now)
	if err != nil {
		return err
	}
	agentService, err := agent.NewService(engine)
	if err != nil {
		return err
	}
	handler := api.NewServer(engine, agentService, report, a.Logger).Handler()
	httpServer := &http.Server{Addr: *address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	backgroundCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var workers sync.WaitGroup
	startWorker := func(name string, run func(context.Context) error) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if workerErr := run(backgroundCtx); workerErr != nil && !errors.Is(workerErr, context.Canceled) {
				a.Logger.Error("background worker stopped", "worker", name, "error", workerErr)
				cancel()
			}
		}()
	}
	onError := func(workerErr error) { a.Logger.Error("background operation failed", "error", workerErr) }
	startWorker("scheduler", rollout.Scheduler{Engine: engine, Interval: defaults.DispatchInterval, OnError: onError}.Run)
	startWorker("lease_reaper", agent.LeaseReaper{Engine: engine, Interval: time.Second, OnError: onError}.Run)
	startWorker("snapshotter", recovery.Snapshotter{Engine: engine, Interval: defaults.SnapshotInterval, OnError: onError}.Run)
	serverErrors := make(chan error, 1)
	go func() {
		a.Logger.Info("HTTP server started", "address", *address, "replayed_events", report.EventsReplayed)
		serverErrors <- httpServer.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
	case serverErr := <-serverErrors:
		if !errors.Is(serverErr, http.ErrServerClosed) {
			cancel()
			workers.Wait()
			return serverErr
		}
	}
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), defaults.ShutdownTimeout)
	defer shutdownCancel()
	shutdownErr := httpServer.Shutdown(shutdownCtx)
	workers.Wait()
	if shutdownErr != nil {
		return fmt.Errorf("HTTP shutdown: %w", shutdownErr)
	}
	return nil
}
