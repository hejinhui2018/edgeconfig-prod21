package rollout

import (
	"time"

	"edgeconfig/domain"
)

type rolloutQueued struct {
	RolloutID domain.RolloutID `json:"rollout_id"`
}
type assignmentCreated struct {
	Assignment domain.Assignment `json:"assignment"`
}
type assignmentLeased struct {
	AssignmentID domain.AssignmentID `json:"assignment_id"`
	LeaseToken   string              `json:"lease_token"`
	ExpiresAt    time.Time           `json:"expires_at"`
}
type deviceProgressed struct {
	Event    domain.DeviceEvent  `json:"event"`
	Status   domain.TargetStatus `json:"status"`
	Attempts int                 `json:"attempts"`
}
type retryScheduled struct {
	RolloutID     domain.RolloutID `json:"rollout_id"`
	DeviceID      domain.DeviceID  `json:"device_id"`
	NextAttemptAt time.Time        `json:"next_attempt_at"`
	LastError     string           `json:"last_error"`
}
type rolloutStatusChanged struct {
	RolloutID domain.RolloutID     `json:"rollout_id"`
	From      domain.RolloutStatus `json:"from"`
	To        domain.RolloutStatus `json:"to"`
}
