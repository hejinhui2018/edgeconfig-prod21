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
	return engine, report, nil
}
