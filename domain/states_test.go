package domain

import (
	"errors"
	"testing"
	"time"
)

func TestTargetTransitionsProtectTerminalState(t *testing.T) {
	path := []TargetStatus{TargetPending, TargetAssigned, TargetAccepted, TargetDownloaded, TargetApplied, TargetHealthChecked}
	for index := 1; index < len(path); index++ {
		if err := ValidateTargetTransition("rollout-a", "device-a", path[index-1], path[index]); err != nil {
			t.Fatalf("valid transition %s -> %s: %v", path[index-1], path[index], err)
		}
	}
	err := ValidateTargetTransition("rollout-a", "device-a", TargetHealthChecked, TargetApplied)
	var transitionErr *TransitionError
	if !errors.As(err, &transitionErr) {
		t.Fatalf("expected contextual TransitionError, got %v", err)
	}
	if transitionErr.RolloutID != "rollout-a" || transitionErr.DeviceID != "device-a" {
		t.Fatalf("missing transition context: %#v", transitionErr)
	}
}

func TestRolloutAggregateCompletesOnlyWhenEveryTargetHealthy(t *testing.T) {
	value, err := NewRollout("rollout-a", "site-a", "config-a", []DeviceID{"one", "two"}, RolloutPolicy{BatchSize: 2, MaxAttempts: 2, LeaseDuration: 1, FailureStrategy: "continue"}, testTime())
	if err != nil {
		t.Fatal(err)
	}
	value.Status = RolloutQueued
	value.Targets["one"].Status = TargetHealthChecked
	value.Targets["two"].Status = TargetApplied
	if got := value.Aggregate(testTime()); got != RolloutApplying {
		t.Fatalf("got %s", got)
	}
	value.Targets["two"].Status = TargetHealthChecked
	if got := value.Aggregate(testTime()); got != RolloutCompleted {
		t.Fatalf("got %s", got)
	}
}

func testTime() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
