package rollout

import (
	"context"
	"fmt"
	"time"

	"edgeconfig/domain"
)

func (e *Engine) PullAssignment(ctx context.Context, deviceID domain.DeviceID) (*domain.Assignment, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.state.Devices[deviceID]; !exists {
		return nil, fmt.Errorf("%w: device %s", ErrNotFound, deviceID)
	}
	now := e.now().UTC()
	var selected *domain.Assignment
	for _, assignment := range e.state.Assignments {
		if assignment.DeviceID != deviceID {
			continue
		}
		rollout := e.state.Rollouts[assignment.RolloutID]
		target := rollout.Targets[deviceID]
		if target.AssignmentID != assignment.ID || target.Status != domain.TargetAssigned {
			continue
		}
		if assignment.Leased(now) {
			continue
		}
		copy := assignment
		if selected == nil || copy.CreatedAt.Before(selected.CreatedAt) {
			selected = &copy
		}
	}
	if selected == nil {
		return nil, nil
	}
	token := domain.NewID("lease", now)
	expires := now.Add(e.state.Rollouts[selected.RolloutID].Policy.LeaseDuration)
	if err := e.appendLocked(ctx, eventSpec{domain.EventAssignmentLeased, string(selected.RolloutID), assignmentLeased{AssignmentID: selected.ID, LeaseToken: token, ExpiresAt: expires}}); err != nil {
		return nil, err
	}
	updated := e.state.Assignments[selected.ID]
	return &updated, nil
}

type EventResult struct {
	Duplicate     bool                 `json:"duplicate"`
	RolloutStatus domain.RolloutStatus `json:"rollout_status"`
	TargetStatus  domain.TargetStatus  `json:"target_status"`
}

func (e *Engine) RecordDeviceEvent(ctx context.Context, event domain.DeviceEvent, leaseToken string) (EventResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.state.ProcessedEvents[event.ID]; exists {
		rollout := e.state.Rollouts[event.RolloutID]
		target := rollout.Targets[event.DeviceID]
		return EventResult{Duplicate: true, RolloutStatus: rollout.Status, TargetStatus: target.Status}, nil
	}
	rolloutValue := e.state.Rollouts[event.RolloutID]
	if rolloutValue == nil {
		return EventResult{}, fmt.Errorf("%w: rollout %s", ErrNotFound, event.RolloutID)
	}
	target := rolloutValue.Targets[event.DeviceID]
	if target == nil {
		return EventResult{}, fmt.Errorf("%w: device %s is not a rollout target", ErrNotFound, event.DeviceID)
	}
	assignment, ok := e.state.Assignments[event.AssignmentID]
	if !ok || assignment.RolloutID != event.RolloutID || assignment.DeviceID != event.DeviceID {
		return EventResult{}, fmt.Errorf("assignment %s does not match rollout and device", event.AssignmentID)
	}
	if target.AssignmentID != assignment.ID {
		return EventResult{}, fmt.Errorf("assignment %s is no longer current", assignment.ID)
	}
	if assignment.LeaseToken == "" || assignment.LeaseToken != leaseToken {
		return EventResult{}, fmt.Errorf("assignment %s lease token is invalid", assignment.ID)
	}
	if event.Sequence <= e.state.DeviceSequences[event.DeviceID] {
		return EventResult{}, fmt.Errorf("%w: sequence %d follows %d", ErrOutOfOrder, event.Sequence, e.state.DeviceSequences[event.DeviceID])
	}
	if event.ObservedAt.IsZero() {
		event.ObservedAt = e.now().UTC()
	}
	destination, err := event.TargetStatus()
	if err != nil {
		return EventResult{}, err
	}
	if event.Kind == domain.EventRejected && !event.Recoverable {
		destination = domain.TargetFailed
	}
	if err := domain.ValidateTargetTransition(event.RolloutID, event.DeviceID, target.Status, destination); err != nil {
		return EventResult{}, err
	}
	specs := []eventSpec{{domain.EventDeviceProgressed, string(event.RolloutID), deviceProgressed{Event: event, Status: destination, Attempts: target.Attempts}}}
	if event.Kind == domain.EventRejected && event.Recoverable && target.Attempts < rolloutValue.Policy.MaxAttempts {
		specs = append(specs, eventSpec{domain.EventTargetRetryScheduled, string(event.RolloutID), retryScheduled{RolloutID: event.RolloutID, DeviceID: event.DeviceID, NextAttemptAt: e.now().Add(rolloutValue.Policy.RetryBackoff), LastError: event.Error}})
	}
	if err := e.appendLocked(ctx, specs...); err != nil {
		return EventResult{}, err
	}
	if err := e.aggregateLocked(ctx, event.RolloutID); err != nil {
		return EventResult{}, err
	}
	current := e.state.Rollouts[event.RolloutID]
	return EventResult{RolloutStatus: current.Status, TargetStatus: current.Targets[event.DeviceID].Status}, nil
}

func (e *Engine) ReapExpiredLeases(ctx context.Context) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := e.now().UTC()
	count := 0
	for _, assignment := range e.state.Assignments {
		if assignment.LeaseToken == "" || assignment.LeaseExpiresAt.After(now) {
			continue
		}
		rolloutValue := e.state.Rollouts[assignment.RolloutID]
		target := rolloutValue.Targets[assignment.DeviceID]
		if target.AssignmentID != assignment.ID || target.Status != domain.TargetAssigned {
			continue
		}
		if err := e.appendLocked(ctx, eventSpec{domain.EventTargetRetryScheduled, string(assignment.RolloutID), retryScheduled{RolloutID: assignment.RolloutID, DeviceID: assignment.DeviceID, NextAttemptAt: now.Add(rolloutValue.Policy.RetryBackoff), LastError: "assignment lease expired"}}); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

var _ = time.Second
