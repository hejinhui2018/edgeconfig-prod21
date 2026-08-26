package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"edgeconfig/agent"
	"edgeconfig/domain"
	"edgeconfig/persistence"
	"edgeconfig/recovery"
	"edgeconfig/rollout"
)

func (a *App) runSmoke(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("smoke", flag.ContinueOnError)
	flags.SetOutput(a.Stderr)
	if err := flags.Parse(args); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "edgeconfig-smoke-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	store, err := persistence.OpenFileStore(dir)
	if err != nil {
		return err
	}
	engine, _, err := recovery.Recover(ctx, store, time.Now)
	if err != nil {
		return err
	}
	agents, _ := agent.NewService(engine)
	now := time.Now().UTC()
	siteID := domain.SiteID("smoke-site")
	deviceID := domain.DeviceID("smoke-device")
	configID := domain.ConfigID("smoke-config")
	rolloutID := domain.RolloutID("smoke-rollout")
	if _, err = engine.CreateSite(ctx, siteID, "Smoke Site", map[string]string{"environment": "local"}); err != nil {
		return err
	}
	if _, err = engine.RegisterDevice(ctx, domain.Device{ID: deviceID, SiteID: siteID, Name: "Smoke Agent", Capabilities: []string{"json-v1"}}); err != nil {
		return err
	}
	parameters := json.RawMessage(`{"poll_seconds":15,"feature_enabled":true}`)
	if _, err = engine.CreateConfiguration(ctx, domain.Configuration{ID: configID, SiteID: siteID, Version: "1.0.0", SchemaVersion: "v1", Parameters: parameters, Validation: domain.ValidationResult{Valid: true}, Summary: "local smoke configuration", RequiredCapabilities: []string{"json-v1"}}); err != nil {
		return err
	}
	policy := rollout.DefaultPolicy()
	policy.BatchSize = 1
	policy.RetryBackoff = 0
	if _, err = engine.CreateRollout(ctx, rollout.CreateRequest{ID: rolloutID, SiteID: siteID, ConfigID: configID, DeviceIDs: []domain.DeviceID{deviceID}, Policy: policy}); err != nil {
		return err
	}
	if count, dispatchErr := engine.Dispatch(ctx); dispatchErr != nil || count != 1 {
		return fmt.Errorf("dispatch assignment: count=%d error=%w", count, dispatchErr)
	}
	assignment, err := agents.Pull(ctx, deviceID)
	if err != nil || assignment == nil {
		return fmt.Errorf("pull assignment: %w", err)
	}
	kinds := []domain.DeviceEventKind{domain.EventAccepted, domain.EventDownloaded, domain.EventApplied, domain.EventHealthChecked}
	for index, kind := range kinds {
		event := domain.DeviceEvent{ID: domain.EventID(fmt.Sprintf("smoke-event-%d", index+1)), RolloutID: rolloutID, AssignmentID: assignment.ID, DeviceID: deviceID, Kind: kind, Sequence: uint64(index + 1), ObservedAt: now.Add(time.Duration(index) * time.Second)}
		if _, err = agents.Report(ctx, event, assignment.LeaseToken); err != nil {
			return fmt.Errorf("report %s: %w", kind, err)
		}
	}
	if err = engine.SaveSnapshot(ctx); err != nil {
		return err
	}
	recovered, report, err := recovery.Recover(ctx, store, time.Now)
	if err != nil {
		return err
	}
	final, err := recovered.GetRollout(rolloutID)
	if err != nil {
		return err
	}
	device, err := recovered.GetDevice(deviceID)
	if err != nil {
		return err
	}
	if final.Status != domain.RolloutCompleted || device.CurrentVersion != "1.0.0" {
		return fmt.Errorf("unexpected recovered state: rollout=%s device_version=%s", final.Status, device.CurrentVersion)
	}
	return json.NewEncoder(a.Stdout).Encode(map[string]any{"status": "ok", "rollout_id": rolloutID, "rollout_status": final.Status, "device_version": device.CurrentVersion, "event_sequence": report.LastSequence, "data_dir_recovered": true})
}
