package persistence

import (
	"context"
	"errors"

	"edgeconfig/domain"
)

var ErrConflict = errors.New("persistence sequence conflict")

type EventStore interface {
	Append(ctx context.Context, expectedSequence uint64, events []domain.Envelope) error
	ReadAfter(ctx context.Context, sequence uint64) ([]domain.Envelope, ReadReport, error)
	LastSequence(ctx context.Context) (uint64, error)
}

type SnapshotStore interface {
	Save(ctx context.Context, snapshot Snapshot) error
	Load(ctx context.Context) (Snapshot, error)
}

type Store interface {
	EventStore
	SnapshotStore
}

type ReadReport struct {
	Records          int    `json:"records"`
	LastSequence     uint64 `json:"last_sequence"`
	IgnoredTailBytes int64  `json:"ignored_tail_bytes"`
}
