package domain

import (
	"fmt"
	"time"
)

type Assignment struct {
	ID             AssignmentID `json:"id"`
	RolloutID      RolloutID    `json:"rollout_id"`
	DeviceID       DeviceID     `json:"device_id"`
	ConfigID       ConfigID     `json:"config_id"`
	ConfigVersion  string       `json:"config_version"`
	SchemaVersion  string       `json:"schema_version"`
	Parameters     []byte       `json:"parameters"`
	Attempt        int          `json:"attempt"`
	LeaseToken     string       `json:"lease_token,omitempty"`
	LeaseExpiresAt time.Time    `json:"lease_expires_at,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
}

func (a Assignment) Leased(now time.Time) bool {
	return a.LeaseToken != "" && a.LeaseExpiresAt.After(now)
}

type DeviceEventKind string

const (
	EventAccepted      DeviceEventKind = "accepted"
	EventDownloaded    DeviceEventKind = "downloaded"
	EventApplied       DeviceEventKind = "applied"
	EventHealthChecked DeviceEventKind = "health_checked"
	EventRejected      DeviceEventKind = "rejected"
)

type DeviceEvent struct {
	ID           EventID         `json:"id"`
	RolloutID    RolloutID       `json:"rollout_id"`
	AssignmentID AssignmentID    `json:"assignment_id"`
	DeviceID     DeviceID        `json:"device_id"`
	Kind         DeviceEventKind `json:"kind"`
	Sequence     uint64          `json:"sequence"`
	Error        string          `json:"error,omitempty"`
	Recoverable  bool            `json:"recoverable,omitempty"`
	ObservedAt   time.Time       `json:"observed_at"`
}

func (e DeviceEvent) TargetStatus() (TargetStatus, error) {
	switch e.Kind {
	case EventAccepted:
		return TargetAccepted, nil
	case EventDownloaded:
		return TargetDownloaded, nil
	case EventApplied:
		return TargetApplied, nil
	case EventHealthChecked:
		return TargetHealthChecked, nil
	case EventRejected:
		return TargetRejected, nil
	default:
		return "", fmt.Errorf("unknown device event kind %q", e.Kind)
	}
}
