package rollout

import (
	"context"
	"encoding/json"
	"fmt"

	"edgeconfig/domain"
	"edgeconfig/persistence"
)

func cloneRollout(value *domain.Rollout) *domain.Rollout {
	if value == nil {
		return nil
	}
	data, _ := json.Marshal(value)
	var copy domain.Rollout
	_ = json.Unmarshal(data, &copy)
	return &copy
}

func (e *Engine) GetRollout(id domain.RolloutID) (*domain.Rollout, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	value := e.state.Rollouts[id]
	if value == nil {
		return nil, fmt.Errorf("%w: rollout %s", ErrNotFound, id)
	}
	return cloneRollout(value), nil
}

func (e *Engine) ListRollouts(siteID domain.SiteID) []*domain.Rollout {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*domain.Rollout, 0)
	for _, value := range e.state.Rollouts {
		if siteID == "" || value.SiteID == siteID {
			result = append(result, cloneRollout(value))
		}
	}
	return result
}

func (e *Engine) GetDevice(id domain.DeviceID) (domain.Device, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	value, ok := e.state.Devices[id]
	if !ok {
		return domain.Device{}, fmt.Errorf("%w: device %s", ErrNotFound, id)
	}
	return value, nil
}

func (e *Engine) Snapshot() (persistence.Snapshot, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	data, err := json.Marshal(e.state)
	if err != nil {
		return persistence.Snapshot{}, err
	}
	var snapshot persistence.Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return persistence.Snapshot{}, err
	}
	return snapshot, snapshot.Normalize()
}

func (e *Engine) SaveSnapshot(ctx context.Context) error {
	snapshot, err := e.Snapshot()
	if err != nil {
		return err
	}
	snapshot.CreatedAt = e.now().UTC()
	return e.store.Save(ctx, snapshot)
}
