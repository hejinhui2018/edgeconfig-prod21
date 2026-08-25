package recovery

import (
	"context"
	"fmt"
	"time"

	"edgeconfig/domain"
	"edgeconfig/persistence"
	"edgeconfig/rollout"
)

type Report struct {
	SnapshotSequence uint64    `json:"snapshot_sequence"`
	EventsReplayed   uint64    `json:"events_replayed"`
	LastSequence     uint64    `json:"last_sequence"`
	IgnoredTailBytes int64     `json:"ignored_tail_bytes"`
	PendingTargets   int       `json:"pending_targets"`
	ActiveLeases     int       `json:"active_leases"`
	RecoveredAt      time.Time `json:"recovered_at"`
}

func Recover(ctx context.Context, store persistence.Store, now rollout.Clock) (*rollout.Engine, Report, error) {
	if now == nil {
		now = time.Now
	}
	snapshot, err := store.Load(ctx)
	if err != nil {
		return nil, Report{}, fmt.Errorf("load snapshot: %w", err)
	}
	engine, err := rollout.NewEngine(store, snapshot, now)
	if err != nil {
		return nil, Report{}, fmt.Errorf("create engine from snapshot: %w", err)
	}
	events, readReport, err := store.ReadAfter(ctx, snapshot.LastSequence)
	if err != nil {
		return nil, Report{}, fmt.Errorf("read events after snapshot %d: %w", snapshot.LastSequence, err)
	}
	previous := snapshot.LastSequence
	for _, event := range events {
		if err := event.Validate(previous); err != nil {
			return nil, Report{}, fmt.Errorf("validate recovery event: %w", err)
		}
		if err := engine.Replay(event); err != nil {
			return nil, Report{}, fmt.Errorf("replay event %s: %w", event.ID, err)
		}
		previous = event.Sequence
	}
	state, err := engine.Snapshot()
	if err != nil {
		return nil, Report{}, err
	}
	report := Report{SnapshotSequence: snapshot.LastSequence, EventsReplayed: uint64(len(events)), LastSequence: state.LastSequence, IgnoredTailBytes: readReport.IgnoredTailBytes, RecoveredAt: now().UTC()}
	for _, value := range state.Rollouts {
		if value.Status.Terminal() {
			continue
		}
		for _, target := range value.Targets {
			if target.Status == domain.TargetPending || target.Status == domain.TargetRejected {
				report.PendingTargets++
			}
		}
	}
	for _, assignment := range state.Assignments {
		if assignment.Leased(now()) {
			report.ActiveLeases++
		}
	}
	// Reconcile rollout aggregation after recovery. Replay only re-applies
	// events to the projection; it does not recompute rollout status, so a
	// rollout whose final device-progress event (e.g. health_checked) was
	// persisted without the trailing EventRolloutStatusChanged event would
	// otherwise stay stuck at its pre-restart status (e.g. applying) even
	// though every target has reached a terminal state. Re-running aggregate
	// for every rollout repersists the missing transition and is a no-op for
	// rollouts whose status already matches their targets (including terminal
	// rollouts). This covers both work waiting for dispatch (pending targets)
	// and work that already finished but whose completion was not durable.
	if err := engine.ReconcileStatuses(ctx); err != nil {
		return nil, Report{}, fmt.Errorf("reconcile recovered rollout statuses: %w", err)
	}
	return engine, report, nil
}
