package rollout_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"

	"edgeconfig/agent"
	"edgeconfig/domain"
	"edgeconfig/internal/testutil"
	"edgeconfig/rollout"
)

func TestDuplicateEventIsAcknowledgedWithoutStateRegression(t *testing.T) {
	fixture := testutil.NewFixture(t)
	policy := rollout.DefaultPolicy()
	fixture.CreateRollout(t, "rollout-a", policy)
	ctx := context.Background()
	if _, err := fixture.Engine.Dispatch(ctx); err != nil {
		t.Fatal(err)
	}
	service, _ := agent.NewService(fixture.Engine)
	assignment, err := service.Pull(ctx, fixture.DeviceID)
	if err != nil {
		t.Fatal(err)
	}
	event := domain.DeviceEvent{ID: "event-accepted", RolloutID: "rollout-a", AssignmentID: assignment.ID, DeviceID: fixture.DeviceID, Kind: domain.EventAccepted, Sequence: 1, ObservedAt: fixture.Now}
	first, err := service.Report(ctx, event, assignment.LeaseToken)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Report(ctx, event, assignment.LeaseToken)
	if err != nil {
		t.Fatal(err)
	}
	if first.Duplicate || !second.Duplicate || second.TargetStatus != domain.TargetAccepted {
		t.Fatalf("unexpected results: first=%+v second=%+v", first, second)
	}
}

func TestOutOfOrderProtocolEventDoesNotAdvanceTarget(t *testing.T) {
	fixture := testutil.NewFixture(t)
	fixture.CreateRollout(t, "rollout-a", rollout.DefaultPolicy())
	ctx := context.Background()
	fixture.Engine.Dispatch(ctx)
	service, _ := agent.NewService(fixture.Engine)
	assignment, _ := service.Pull(ctx, fixture.DeviceID)
	event := domain.DeviceEvent{ID: "event-applied", RolloutID: "rollout-a", AssignmentID: assignment.ID, DeviceID: fixture.DeviceID, Kind: domain.EventApplied, Sequence: 1, ObservedAt: fixture.Now}
	_, err := service.Report(ctx, event, assignment.LeaseToken)
	var transition *domain.TransitionError
	if !errors.As(err, &transition) {
		t.Fatalf("expected transition error, got %v", err)
	}
	value, _ := fixture.Engine.GetRollout("rollout-a")
	if value.Targets[fixture.DeviceID].Status != domain.TargetAssigned {
		t.Fatalf("target advanced to %s", value.Targets[fixture.DeviceID].Status)
	}
}

func TestHealthyTargetRecordsVerifiedBeforeCompleted(t *testing.T) {
	fixture := testutil.NewFixture(t)
	fixture.CreateRollout(t, "rollout-complete", rollout.DefaultPolicy())
	ctx := context.Background()
	fixture.Engine.Dispatch(ctx)
	service, _ := agent.NewService(fixture.Engine)
	assignment, _ := service.Pull(ctx, fixture.DeviceID)
	kinds := []domain.DeviceEventKind{domain.EventAccepted, domain.EventDownloaded, domain.EventApplied, domain.EventHealthChecked}
	for index, kind := range kinds {
		event := domain.DeviceEvent{ID: domain.EventID(fmt.Sprintf("complete-event-%d", index)), RolloutID: "rollout-complete", AssignmentID: assignment.ID, DeviceID: fixture.DeviceID, Kind: kind, Sequence: uint64(index + 1), ObservedAt: fixture.Now}
		if _, err := service.Report(ctx, event, assignment.LeaseToken); err != nil {
			t.Fatal(err)
		}
	}
	events, _, err := fixture.Store.ReadAfter(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	statuses := make([]domain.RolloutStatus, 0, 2)
	for _, event := range events {
		if event.Type != domain.EventRolloutStatusChanged {
			continue
		}
		var change struct {
			To domain.RolloutStatus `json:"to"`
		}
		if err := json.Unmarshal(event.Data, &change); err != nil {
			t.Fatal(err)
		}
		if change.To == domain.RolloutVerified || change.To == domain.RolloutCompleted {
			statuses = append(statuses, change.To)
		}
	}
	if len(statuses) < 2 || !slices.Equal(statuses[len(statuses)-2:], []domain.RolloutStatus{domain.RolloutVerified, domain.RolloutCompleted}) {
		t.Fatalf("final status events: %v", statuses)
	}
}
