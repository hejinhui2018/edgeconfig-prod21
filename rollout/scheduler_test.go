package rollout_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"edgeconfig/internal/testutil"
	"edgeconfig/rollout"
)

func TestSchedulerStopsWhenContextIsCancelled(t *testing.T) {
	fixture := testutil.NewFixture(t)
	fixture.CreateRollout(t, "rollout-scheduled", rollout.DefaultPolicy())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- (rollout.Scheduler{Engine: fixture.Engine, Interval: time.Millisecond}).Run(ctx) }()
	deadline := time.Now().Add(time.Second)
	for {
		value, _ := fixture.Engine.GetRollout("rollout-scheduled")
		if value.Targets[fixture.DeviceID].Status == "assigned" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("scheduler did not dispatch")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop")
	}
}
