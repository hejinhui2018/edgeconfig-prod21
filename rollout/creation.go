package rollout

import (
	"context"
	"fmt"
	"slices"

	"edgeconfig/domain"
)

type CreateRequest struct {
	ID        domain.RolloutID
	SiteID    domain.SiteID
	ConfigID  domain.ConfigID
	DeviceIDs []domain.DeviceID
	Selector  map[string]string
	Policy    domain.RolloutPolicy
}

func (e *Engine) CreateRollout(ctx context.Context, request CreateRequest) (*domain.Rollout, error) {
	e.mu.Lock()
	if _, exists := e.state.Rollouts[request.ID]; exists {
		e.mu.Unlock()
		return nil, fmt.Errorf("%w: rollout %s", ErrAlreadyExists, request.ID)
	}
	configuration, exists := e.state.Configurations[request.ConfigID]
	if !exists {
		e.mu.Unlock()
		return nil, fmt.Errorf("%w: configuration %s", ErrNotFound, request.ConfigID)
	}
	if configuration.SiteID != request.SiteID {
		e.mu.Unlock()
		return nil, fmt.Errorf("configuration %s belongs to site %s, not %s", request.ConfigID, configuration.SiteID, request.SiteID)
	}
	selected := make([]domain.DeviceID, 0)
	for id, device := range e.state.Devices {
		if device.SiteID != request.SiteID {
			continue
		}
		if len(request.DeviceIDs) > 0 && !slices.Contains(request.DeviceIDs, id) {
			continue
		}
		if !matches(device.Labels, request.Selector) {
			continue
		}
		if !device.Supports(configuration.RequiredCapabilities) {
			continue
		}
		selected = append(selected, id)
	}
	slices.Sort(selected)
	rolloutValue, err := domain.NewRollout(request.ID, request.SiteID, request.ConfigID, selected, request.Policy, e.now())
	if err != nil {
		e.mu.Unlock()
		return nil, err
	}
	e.mu.Unlock()
	queued := rolloutQueued{RolloutID: rolloutValue.ID}
	if err := e.appendLocked(ctx,
		eventSpec{domain.EventRolloutCreated, string(request.ID), *rolloutValue},
		eventSpec{domain.EventRolloutQueued, string(request.ID), queued},
	); err != nil {
		return nil, err
	}
	return cloneRollout(e.state.Rollouts[request.ID]), nil
}

func matches(labels, selector map[string]string) bool {
	for key, expected := range selector {
		if labels[key] != expected {
			return false
		}
	}
	return true
}
