package domain

import (
	"fmt"
	"slices"
)

type RolloutStatus string

const (
	RolloutDraft       RolloutStatus = "draft"
	RolloutQueued      RolloutStatus = "queued"
	RolloutDispatching RolloutStatus = "dispatching"
	RolloutApplying    RolloutStatus = "applying"
	RolloutVerified    RolloutStatus = "verified"
	RolloutCompleted   RolloutStatus = "completed"
	RolloutFailed      RolloutStatus = "failed"
)

type TargetStatus string

const (
	TargetPending       TargetStatus = "pending"
	TargetAssigned      TargetStatus = "assigned"
	TargetAccepted      TargetStatus = "accepted"
	TargetDownloaded    TargetStatus = "downloaded"
	TargetApplied       TargetStatus = "applied"
	TargetHealthChecked TargetStatus = "health_checked"
	TargetRejected      TargetStatus = "rejected"
	TargetFailed        TargetStatus = "failed"
)

var targetRanks = map[TargetStatus]int{
	TargetPending: 0, TargetAssigned: 1, TargetAccepted: 2, TargetDownloaded: 3,
	TargetApplied: 4, TargetHealthChecked: 5, TargetRejected: 6, TargetFailed: 7,
}

func (s TargetStatus) Terminal() bool {
	return s == TargetHealthChecked || s == TargetFailed
}

func (s RolloutStatus) Terminal() bool {
	return s == RolloutCompleted || s == RolloutFailed
}

type TransitionError struct {
	RolloutID RolloutID
	DeviceID  DeviceID
	From      TargetStatus
	To        TargetStatus
	Reason    string
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("rollout %s device %s cannot transition from %s to %s: %s", e.RolloutID, e.DeviceID, e.From, e.To, e.Reason)
}

func ValidateTargetTransition(rolloutID RolloutID, deviceID DeviceID, from, to TargetStatus) error {
	if _, ok := targetRanks[from]; !ok {
		return &TransitionError{rolloutID, deviceID, from, to, "unknown current status"}
	}
	if _, ok := targetRanks[to]; !ok {
		return &TransitionError{rolloutID, deviceID, from, to, "unknown requested status"}
	}
	if from == to {
		return nil
	}
	if from.Terminal() {
		return &TransitionError{rolloutID, deviceID, from, to, "terminal device state cannot change"}
	}
	allowed := map[TargetStatus][]TargetStatus{
		TargetPending:    {TargetAssigned, TargetFailed},
		TargetAssigned:   {TargetAccepted, TargetRejected, TargetFailed},
		TargetAccepted:   {TargetDownloaded, TargetRejected, TargetFailed},
		TargetDownloaded: {TargetApplied, TargetRejected, TargetFailed},
		TargetApplied:    {TargetHealthChecked, TargetRejected, TargetFailed},
		TargetRejected:   {TargetPending, TargetAssigned, TargetFailed},
	}
	if !slices.Contains(allowed[from], to) {
		return &TransitionError{rolloutID, deviceID, from, to, "transition is not part of the device protocol"}
	}
	return nil
}
