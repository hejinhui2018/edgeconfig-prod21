package domain

import (
	"fmt"
	"time"
)

type RolloutPolicy struct {
	BatchSize       int           `json:"batch_size"`
	MaxAttempts     int           `json:"max_attempts"`
	LeaseDuration   time.Duration `json:"lease_duration"`
	RetryBackoff    time.Duration `json:"retry_backoff"`
	FailureStrategy string        `json:"failure_strategy"`
}

func (p RolloutPolicy) Validate() error {
	if p.BatchSize < 1 || p.BatchSize > 1000 {
		return fmt.Errorf("batch_size must be between 1 and 1000")
	}
	if p.MaxAttempts < 1 || p.MaxAttempts > 20 {
		return fmt.Errorf("max_attempts must be between 1 and 20")
	}
	if p.LeaseDuration <= 0 {
		return fmt.Errorf("lease_duration must be positive")
	}
	if p.RetryBackoff < 0 {
		return fmt.Errorf("retry_backoff cannot be negative")
	}
	if p.FailureStrategy != "fail_fast" && p.FailureStrategy != "continue" {
		return fmt.Errorf("failure_strategy must be fail_fast or continue")
	}
	return nil
}

type RolloutTarget struct {
	DeviceID      DeviceID     `json:"device_id"`
	Status        TargetStatus `json:"status"`
	Attempts      int          `json:"attempts"`
	LastError     string       `json:"last_error,omitempty"`
	LastEventID   EventID      `json:"last_event_id,omitempty"`
	LastEventAt   time.Time    `json:"last_event_at,omitempty"`
	NextAttemptAt time.Time    `json:"next_attempt_at,omitempty"`
	AssignmentID  AssignmentID `json:"assignment_id,omitempty"`
}

type Rollout struct {
	ID        RolloutID                   `json:"id"`
	SiteID    SiteID                      `json:"site_id"`
	ConfigID  ConfigID                    `json:"config_id"`
	Status    RolloutStatus               `json:"status"`
	Policy    RolloutPolicy               `json:"policy"`
	Targets   map[DeviceID]*RolloutTarget `json:"targets"`
	CreatedAt time.Time                   `json:"created_at"`
	UpdatedAt time.Time                   `json:"updated_at"`
}

func NewRollout(id RolloutID, siteID SiteID, configID ConfigID, devices []DeviceID, policy RolloutPolicy, now time.Time) (*Rollout, error) {
	if err := ValidateID("rollout", string(id)); err != nil {
		return nil, err
	}
	if err := policy.Validate(); err != nil {
		return nil, fmt.Errorf("rollout %s policy: %w", id, err)
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("rollout %s requires at least one device", id)
	}
	targets := make(map[DeviceID]*RolloutTarget, len(devices))
	for _, deviceID := range devices {
		if err := ValidateID("device", string(deviceID)); err != nil {
			return nil, err
		}
		if _, duplicate := targets[deviceID]; duplicate {
			return nil, fmt.Errorf("rollout %s contains duplicate device %s", id, deviceID)
		}
		targets[deviceID] = &RolloutTarget{DeviceID: deviceID, Status: TargetPending}
	}
	now = now.UTC()
	return &Rollout{ID: id, SiteID: siteID, ConfigID: configID, Status: RolloutDraft, Policy: policy, Targets: targets, CreatedAt: now, UpdatedAt: now}, nil
}

func (r *Rollout) Aggregate(now time.Time) RolloutStatus {
	if r.Status.Terminal() {
		return r.Status
	}
	var pending, active, applied, healthy, failed int
	for _, target := range r.Targets {
		switch target.Status {
		case TargetPending:
			pending++
		case TargetAssigned, TargetAccepted, TargetDownloaded:
			active++
		case TargetApplied:
			applied++
		case TargetHealthChecked:
			healthy++
		case TargetFailed:
			failed++
		case TargetRejected:
			if target.Attempts >= r.Policy.MaxAttempts {
				failed++
			} else {
				pending++
			}
		}
	}
	next := r.Status
	switch {
	case failed > 0 && (r.Policy.FailureStrategy == "fail_fast" || healthy+failed == len(r.Targets)):
		next = RolloutFailed
	case healthy == len(r.Targets):
		next = RolloutCompleted
	case healthy > 0 && pending == 0 && active == 0 && applied == 0:
		next = RolloutVerified
	case applied > 0 || healthy > 0:
		next = RolloutApplying
	case active > 0:
		next = RolloutDispatching
	case r.Status != RolloutDraft:
		next = RolloutQueued
	}
	if next != r.Status {
		r.Status = next
		r.UpdatedAt = now.UTC()
	}
	return r.Status
}
