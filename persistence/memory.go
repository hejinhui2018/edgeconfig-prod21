package persistence

import (
	"context"
	"fmt"
	"sync"

	"edgeconfig/domain"
)

type MemoryStore struct {
	mu       sync.RWMutex
	events   []domain.Envelope
	snapshot Snapshot
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{snapshot: EmptySnapshot()} }

func (s *MemoryStore) Append(ctx context.Context, expected uint64, events []domain.Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	actual := uint64(len(s.events))
	if actual != expected {
		return fmt.Errorf("%w: expected %d, actual %d", ErrConflict, expected, actual)
	}
	previous := expected
	for _, event := range events {
		if err := event.Validate(previous); err != nil {
			return err
		}
		previous = event.Sequence
	}
	s.events = append(s.events, events...)
	return nil
}

func (s *MemoryStore) ReadAfter(ctx context.Context, sequence uint64) ([]domain.Envelope, ReadReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return nil, ReadReport{}, err
	}
	start := len(s.events)
	for index, event := range s.events {
		if event.Sequence > sequence {
			start = index
			break
		}
	}
	copy := append([]domain.Envelope(nil), s.events[start:]...)
	return copy, ReadReport{Records: len(s.events), LastSequence: uint64(len(s.events))}, nil
}

func (s *MemoryStore) LastSequence(ctx context.Context) (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return uint64(len(s.events)), nil
}

func (s *MemoryStore) Save(ctx context.Context, snapshot Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := snapshot.Normalize(); err != nil {
		return err
	}
	data, err := cloneSnapshot(snapshot)
	if err != nil {
		return err
	}
	s.snapshot = data
	return nil
}

func (s *MemoryStore) Load(ctx context.Context) (Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	return cloneSnapshot(s.snapshot)
}
