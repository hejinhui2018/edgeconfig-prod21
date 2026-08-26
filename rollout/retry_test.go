package rollout_test

import (
	"context"
	"testing"

	"edgeconfig/agent"
	"edgeconfig/domain"
	"edgeconfig/internal/testutil"
	"edgeconfig/rollout"
)

func TestRecoverableRejectionSchedulesAnotherAttempt(t *testing.T) {
	fixture := testutil.NewFixture(t)
	policy := rollout.DefaultPolicy()
	policy.MaxAttempts = 2
	policy.RetryBackoff = 0
	fixture.CreateRollout(t, "rollout-retry", policy)
	ctx := context.Background()
	fixture.Engine.Dispatch(ctx)
	service, _ := agent.NewService(fixture.Engine)
	first, _ := service.Pull(ctx, fixture.DeviceID)
	rejected := domain.DeviceEvent{ID: "event-rejected", RolloutID: "rollout-retry", AssignmentID: first.ID, DeviceID: fixture.DeviceID, Kind: domain.EventRejected, Sequence: 1, Error: "download timeout", Recoverable: true, ObservedAt: fixture.Now}
	result, err := service.Report(ctx, rejected, first.LeaseToken)
	if err != nil {
		t.Fatal(err)
	}
	if result.TargetStatus != domain.TargetPending {
		t.Fatalf("got %s", result.TargetStatus)
	}
	if count, err := fixture.Engine.Dispatch(ctx); err != nil || count != 1 {
		t.Fatalf("retry dispatch count=%d err=%v", count, err)
	}
	second, _ := service.Pull(ctx, fixture.DeviceID)
	if second.Attempt != 2 || second.ID == first.ID {
		t.Fatalf("unexpected retry assignment: %+v", second)
	}
}

func TestUnrecoverableRejectionFailsRollout(t *testing.T) {
	fixture := testutil.NewFixture(t)
	fixture.CreateRollout(t, "rollout-fail", rollout.DefaultPolicy())
	ctx := context.Background()
	fixture.Engine.Dispatch(ctx)
	service, _ := agent.NewService(fixture.Engine)
	assignment, _ := service.Pull(ctx, fixture.DeviceID)
	event := domain.DeviceEvent{ID: "event-fatal", RolloutID: "rollout-fail", AssignmentID: assignment.ID, DeviceID: fixture.DeviceID, Kind: domain.EventRejected, Sequence: 1, Error: "invalid signature", ObservedAt: fixture.Now}
	result, err := service.Report(ctx, event, assignment.LeaseToken)
	if err != nil {
		t.Fatal(err)
	}
	if result.RolloutStatus != domain.RolloutFailed || result.TargetStatus != domain.TargetFailed {
		t.Fatalf("unexpected result %+v", result)
	}
}
