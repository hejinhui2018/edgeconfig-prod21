package recovery

import (
	"context"
	"fmt"

	"edgeconfig/persistence"
)

type Verification struct {
	Events           int    `json:"events"`
	LastSequence     uint64 `json:"last_sequence"`
	SnapshotSequence uint64 `json:"snapshot_sequence"`
}

func Verify(ctx context.Context, store persistence.Store) (Verification, error) {
	snapshot, err := store.Load(ctx)
	if err != nil {
		return Verification{}, err
	}
	events, report, err := store.ReadAfter(ctx, 0)
	if err != nil {
		return Verification{}, err
	}
	if snapshot.LastSequence > report.LastSequence {
		return Verification{}, fmt.Errorf("snapshot sequence %d exceeds log sequence %d", snapshot.LastSequence, report.LastSequence)
	}
	var previous uint64
	for _, event := range events {
		if err := event.Validate(previous); err != nil {
			return Verification{}, err
		}
		previous = event.Sequence
	}
	return Verification{Events: len(events), LastSequence: report.LastSequence, SnapshotSequence: snapshot.LastSequence}, nil
}
