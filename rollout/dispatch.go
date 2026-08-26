package rollout

import (
	"context"
	"fmt"
	"slices"
	"time"

	"edgeconfig/domain"
)

func (e *Engine) Dispatch(ctx context.Context) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := e.now().UTC()
	rolloutIDs := make([]domain.RolloutID, 0, len(e.state.Rollouts))
	for id := range e.state.Rollouts {
		rolloutIDs = append(rolloutIDs, id)
	}
	slices.Sort(rolloutIDs)
	created := 0
	for _, rolloutID := range rolloutIDs {
		value := e.state.Rollouts[rolloutID]
		if value.Status.Terminal() || value.Status == domain.RolloutDraft {
			continue
		}
		active := 0
		for _, target := range value.Targets {
			if target.Status == domain.TargetAssigned || target.Status == domain.TargetAccepted || target.Status == domain.TargetDownloaded || target.Status == domain.TargetApplied {
				active++
			}
		}
		capacity := value.Policy.BatchSize - active
		if capacity <= 0 {
			continue
		}
		deviceIDs := make([]domain.DeviceID, 0, len(value.Targets))
		for id := range value.Targets {
			deviceIDs = append(deviceIDs, id)
		}
		slices.Sort(deviceIDs)
		for _, deviceID := range deviceIDs {
			if capacity == 0 {
				break
			}
			target := value.Targets[deviceID]
			if target.Status != domain.TargetPending || target.NextAttemptAt.After(now) {
				continue
			}
			if target.Attempts >= value.Policy.MaxAttempts {
				failureEvent := domain.DeviceEvent{ID: domain.EventID(domain.NewID("budget", now)), RolloutID: value.ID, DeviceID: deviceID, Kind: domain.EventRejected, Sequence: e.state.DeviceSequences[deviceID] + 1, Error: "retry budget exhausted", ObservedAt: now}
				if err := e.appendLocked(ctx, eventSpec{domain.EventDeviceProgressed, string(value.ID), deviceProgressed{Event: failureEvent, Status: domain.TargetFailed, Attempts: target.Attempts}}); err != nil {
					return created, err
				}
				continue
			}
			configuration := e.state.Configurations[value.ConfigID]
			assignment := domain.Assignment{ID: domain.AssignmentID(domain.NewID("asg", now)), RolloutID: value.ID, DeviceID: deviceID, ConfigID: configuration.ID, ConfigVersion: configuration.Version, SchemaVersion: configuration.SchemaVersion, Parameters: append([]byte(nil), configuration.Parameters...), Attempt: target.Attempts + 1, CreatedAt: now}
			if err := e.appendLocked(ctx, eventSpec{domain.EventAssignmentCreated, string(value.ID), assignmentCreated{Assignment: assignment}}); err != nil {
				return created, err
			}
			created++
			capacity--
		}
		if err := e.aggregateLocked(ctx, value.ID); err != nil {
			return created, err
		}
	}
	return created, nil
}

func (e *Engine) aggregateLocked(ctx context.Context, id domain.RolloutID) error {
	value := e.state.Rollouts[id]
	if value == nil {
		return fmt.Errorf("%w: rollout %s", ErrNotFound, id)
	}
	before := value.Status
	allHealthy := len(value.Targets) > 0
	for _, target := range value.Targets {
		if target.Status != domain.TargetHealthChecked {
			allHealthy = false
			break
		}
	}
	if allHealthy && before != domain.RolloutCompleted {
		specs := make([]eventSpec, 0, 2)
		if before != domain.RolloutVerified {
			specs = append(specs, eventSpec{domain.EventRolloutStatusChanged, string(id), rolloutStatusChanged{RolloutID: id, From: before, To: domain.RolloutVerified}})
			before = domain.RolloutVerified
		}
		specs = append(specs, eventSpec{domain.EventRolloutStatusChanged, string(id), rolloutStatusChanged{RolloutID: id, From: before, To: domain.RolloutCompleted}})
		return e.appendLocked(ctx, specs...)
	}
	copy := cloneRollout(value)
	after := copy.Aggregate(e.now())
	if after == before {
		return nil
	}
	return e.appendLocked(ctx, eventSpec{domain.EventRolloutStatusChanged, string(id), rolloutStatusChanged{RolloutID: id, From: before, To: after}})
}

type Scheduler struct {
	Engine   *Engine
	Interval time.Duration
	OnError  func(error)
}

func (s Scheduler) Run(ctx context.Context) error {
	if s.Engine == nil {
		return fmt.Errorf("scheduler engine is required")
	}
	if s.Interval <= 0 {
		s.Interval = 250 * time.Millisecond
	}
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for {
		if _, err := s.Engine.Dispatch(ctx); err != nil && ctx.Err() == nil {
			if s.OnError != nil {
				s.OnError(err)
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
