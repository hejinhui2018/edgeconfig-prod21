package agent

import (
	"context"
	"fmt"
	"strings"

	"edgeconfig/domain"
	"edgeconfig/rollout"
)

type Service struct{ engine *rollout.Engine }

func NewService(engine *rollout.Engine) (*Service, error) {
	if engine == nil {
		return nil, fmt.Errorf("rollout engine is required")
	}
	return &Service{engine: engine}, nil
}

func (s *Service) Pull(ctx context.Context, deviceID domain.DeviceID) (*domain.Assignment, error) {
	if err := domain.ValidateID("device", string(deviceID)); err != nil {
		return nil, err
	}
	return s.engine.PullAssignment(ctx, deviceID)
}

func (s *Service) Report(ctx context.Context, event domain.DeviceEvent, leaseToken string) (rollout.EventResult, error) {
	if err := validateEvent(event, leaseToken); err != nil {
		return rollout.EventResult{}, err
	}
	return s.engine.RecordDeviceEvent(ctx, event, leaseToken)
}

func validateEvent(event domain.DeviceEvent, leaseToken string) error {
	if err := domain.ValidateID("event", string(event.ID)); err != nil {
		return err
	}
	if err := domain.ValidateID("rollout", string(event.RolloutID)); err != nil {
		return err
	}
	if err := domain.ValidateID("assignment", string(event.AssignmentID)); err != nil {
		return err
	}
	if err := domain.ValidateID("device", string(event.DeviceID)); err != nil {
		return err
	}
	if strings.TrimSpace(leaseToken) == "" {
		return fmt.Errorf("lease token is required")
	}
	if event.Sequence == 0 {
		return fmt.Errorf("device event sequence must be positive")
	}
	if _, err := event.TargetStatus(); err != nil {
		return err
	}
	if event.Kind == domain.EventRejected && strings.TrimSpace(event.Error) == "" {
		return fmt.Errorf("rejected event requires an error")
	}
	return nil
}
