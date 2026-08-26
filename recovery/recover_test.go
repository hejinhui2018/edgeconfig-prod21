package recovery_test

import (
	"context"
	"testing"
	"time"

	"edgeconfig/internal/testutil"
	"edgeconfig/recovery"
	"edgeconfig/rollout"
)

func TestRecoveryUsesSnapshotThenReplaysNewEvents(t *testing.T) {
	fixture := testutil.NewFixture(t)
	ctx := context.Background()
	if err := fixture.Engine.SaveSnapshot(ctx); err != nil {
		t.Fatal(err)
	}
	fixture.CreateRollout(t, "rollout-after-snapshot", rollout.DefaultPolicy())
	recovered, report, err := recovery.Recover(ctx, fixture.Store, func() time.Time { return fixture.Now })
	if err != nil {
		t.Fatal(err)
	}
	if report.SnapshotSequence != 3 || report.EventsReplayed != 2 {
		t.Fatalf("unexpected report %+v", report)
	}
	value, err := recovered.GetRollout("rollout-after-snapshot")
	if err != nil || value.Status != "queued" {
		t.Fatalf("rollout=%+v err=%v", value, err)
	}
}
