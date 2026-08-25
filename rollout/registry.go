package rollout

import (
	"context"
	"fmt"
	"time"

	"edgeconfig/domain"
)

func (e *Engine) CreateSite(ctx context.Context, id domain.SiteID, name string, labels map[string]string) (domain.Site, error) {
	site, err := domain.NewSite(id, name, labels, e.now())
	if err != nil {
		return domain.Site{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.state.Sites[id]; exists {
		return domain.Site{}, fmt.Errorf("%w: site %s", ErrAlreadyExists, id)
	}
	err = e.appendLocked(ctx, eventSpec{domain.EventSiteCreated, string(id), site})
	return site, err
}

func (e *Engine) RegisterDevice(ctx context.Context, value domain.Device) (domain.Device, error) {
	device, err := domain.NewDevice(value.ID, value.SiteID, value.Name, value.Capabilities, value.Labels, e.now())
	if err != nil {
		return domain.Device{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.state.Sites[device.SiteID]; !exists {
		return domain.Device{}, fmt.Errorf("%w: site %s", ErrNotFound, device.SiteID)
	}
	if _, exists := e.state.Devices[device.ID]; exists {
		return domain.Device{}, fmt.Errorf("%w: device %s", ErrAlreadyExists, device.ID)
	}
	err = e.appendLocked(ctx, eventSpec{domain.EventDeviceRegistered, string(device.ID), device})
	return device, err
}

func (e *Engine) CreateConfiguration(ctx context.Context, value domain.Configuration) (domain.Configuration, error) {
	configuration, err := domain.NewConfiguration(value, e.now())
	if err != nil {
		return domain.Configuration{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.state.Sites[configuration.SiteID]; !exists {
		return domain.Configuration{}, fmt.Errorf("%w: site %s", ErrNotFound, configuration.SiteID)
	}
	if _, exists := e.state.Configurations[configuration.ID]; exists {
		return domain.Configuration{}, fmt.Errorf("%w: configuration %s", ErrAlreadyExists, configuration.ID)
	}
	err = e.appendLocked(ctx, eventSpec{domain.EventConfigurationCreated, string(configuration.ID), configuration})
	return configuration, err
}

func DefaultPolicy() domain.RolloutPolicy {
	return domain.RolloutPolicy{BatchSize: 10, MaxAttempts: 3, LeaseDuration: 30 * time.Second, RetryBackoff: 5 * time.Second, FailureStrategy: "fail_fast"}
}
