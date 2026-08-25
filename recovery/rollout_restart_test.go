package recovery

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"edgeconfig/agent"
	"edgeconfig/domain"
	"edgeconfig/persistence"
	"edgeconfig/rollout"
)

func TestRolloutCompletesAfterRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := persistence.OpenFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	engine, _, err := Recover(ctx, store, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	siteID := domain.SiteID("restart-site")
	deviceID := domain.DeviceID("restart-device")
	configID := domain.ConfigID("restart-config")
	rolloutID := domain.RolloutID("restart-rollout")
	if _, err = engine.CreateSite(ctx, siteID, "Restart Site", nil); err != nil {
		t.Fatal(err)
	}
	if _, err = engine.RegisterDevice(ctx, domain.Device{ID: deviceID, SiteID: siteID, Name: "Restart Device", Capabilities: []string{"json-v1"}}); err != nil {
		t.Fatal(err)
	}
	if _, err = engine.CreateConfiguration(ctx, domain.Configuration{
		ID: configID, SiteID: siteID, Version: "2.0.0", SchemaVersion: "v1",
		Parameters: json.RawMessage(`{"feature_enabled":true}`),
		Validation: domain.ValidationResult{Valid: true}, Summary: "restart validation",
		RequiredCapabilities: []string{"json-v1"},
	}); err != nil {
		t.Fatal(err)
	}
	policy := rollout.DefaultPolicy()
	policy.BatchSize = 1
	policy.RetryBackoff = 0
	if _, err = engine.CreateRollout(ctx, rollout.CreateRequest{ID: rolloutID, SiteID: siteID, ConfigID: configID, DeviceIDs: []domain.DeviceID{deviceID}, Policy: policy}); err != nil {
		t.Fatal(err)
	}
	if count, err := engine.Dispatch(ctx); err != nil || count != 1 {
		t.Fatalf("dispatch count=%d err=%v", count, err)
	}
	service, err := agent.NewService(engine)
	if err != nil {
		t.Fatal(err)
	}
	assignment, err := service.Pull(ctx, deviceID)
	if err != nil || assignment == nil {
		t.Fatalf("pull assignment=%v err=%v", assignment, err)
	}
	for sequence, kind := range []domain.DeviceEventKind{domain.EventAccepted, domain.EventDownloaded, domain.EventApplied} {
		if _, err = service.Report(ctx, domain.DeviceEvent{ID: domain.EventID("restart-event-" + string(rune('a'+sequence))), RolloutID: rolloutID, AssignmentID: assignment.ID, DeviceID: deviceID, Kind: kind, Sequence: uint64(sequence + 1), ObservedAt: now}, assignment.LeaseToken); err != nil {
			t.Fatal(err)
		}
	}
	if err = engine.SaveSnapshot(ctx); err != nil {
		t.Fatal(err)
	}
	snapshot, err := engine.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Report(ctx, domain.DeviceEvent{ID: "restart-event-d", RolloutID: rolloutID, AssignmentID: assignment.ID, DeviceID: deviceID, Kind: domain.EventHealthChecked, Sequence: 4, ObservedAt: now}, assignment.LeaseToken); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitAfter(string(data), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event domain.Envelope
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event.Sequence <= snapshot.LastSequence+1 {
			kept = append(kept, line)
		}
	}
	if err = os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(strings.Join(kept, "")), 0o640); err != nil {
		t.Fatal(err)
	}
	reopened, err := persistence.OpenFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	recovered, _, err := Recover(ctx, reopened, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	value, err := recovered.GetRollout(rolloutID)
	if err != nil {
		t.Fatal(err)
	}
	if value.Status != domain.RolloutCompleted {
		t.Fatalf("rollout status after restart = %s, want completed", value.Status)
	}
}
