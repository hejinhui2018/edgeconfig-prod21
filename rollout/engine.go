package rollout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"edgeconfig/domain"
	"edgeconfig/persistence"
)

var (
	ErrNotFound       = errors.New("resource not found")
	ErrAlreadyExists  = errors.New("resource already exists")
	ErrDuplicateEvent = errors.New("device event already processed")
	ErrOutOfOrder     = errors.New("device event is out of order")
)

type Clock func() time.Time

type Engine struct {
	mu    sync.RWMutex
	store persistence.Store
	state persistence.Snapshot
	now   Clock
}

func NewEngine(store persistence.Store, snapshot persistence.Snapshot, now Clock) (*Engine, error) {
	if store == nil {
		return nil, fmt.Errorf("event store is required")
	}
	if err := snapshot.Normalize(); err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	return &Engine{store: store, state: snapshot, now: now}, nil
}

func (e *Engine) appendLocked(ctx context.Context, specs ...eventSpec) error {
	if len(specs) == 0 {
		return nil
	}
	events := make([]domain.Envelope, 0, len(specs))
	sequence := e.state.LastSequence
	for _, spec := range specs {
		sequence++
		envelope, err := domain.NewEnvelope(sequence, domain.EventID(domain.NewID("evt", e.now())), spec.kind, spec.aggregateID, spec.data, e.now())
		if err != nil {
			return err
		}
		events = append(events, envelope)
	}
	if err := e.store.Append(ctx, e.state.LastSequence, events); err != nil {
		return err
	}
	for _, event := range events {
		if err := apply(&e.state, event); err != nil {
			return fmt.Errorf("apply committed event %s: %w", event.ID, err)
		}
	}
	return nil
}

type eventSpec struct {
	kind        domain.EventType
	aggregateID string
	data        any
}

func decode[T any](event domain.Envelope) (T, error) {
	var value T
	if err := json.Unmarshal(event.Data, &value); err != nil {
		return value, fmt.Errorf("decode %s event %s: %w", event.Type, event.ID, err)
	}
	return value, nil
}

func (e *Engine) Replay(event domain.Envelope) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return apply(&e.state, event)
}

func (e *Engine) LastSequence() uint64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.LastSequence
}
