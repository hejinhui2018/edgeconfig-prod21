package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

const EventSchemaVersion = 1

type EventType string

const (
	EventSiteCreated          EventType = "site.created"
	EventDeviceRegistered     EventType = "device.registered"
	EventConfigurationCreated EventType = "configuration.created"
	EventRolloutCreated       EventType = "rollout.created"
	EventRolloutQueued        EventType = "rollout.queued"
	EventAssignmentCreated    EventType = "assignment.created"
	EventAssignmentLeased     EventType = "assignment.leased"
	EventDeviceProgressed     EventType = "device.progressed"
	EventTargetRetryScheduled EventType = "target.retry_scheduled"
	EventRolloutStatusChanged EventType = "rollout.status_changed"
)

type Envelope struct {
	SchemaVersion int             `json:"schema_version"`
	Sequence      uint64          `json:"sequence"`
	ID            EventID         `json:"id"`
	Type          EventType       `json:"type"`
	AggregateID   string          `json:"aggregate_id"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Data          json.RawMessage `json:"data"`
}

func NewEnvelope(sequence uint64, id EventID, eventType EventType, aggregateID string, data any, now time.Time) (Envelope, error) {
	if sequence == 0 {
		return Envelope{}, fmt.Errorf("event sequence must be positive")
	}
	if err := ValidateID("event", string(id)); err != nil {
		return Envelope{}, err
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal %s event data: %w", eventType, err)
	}
	return Envelope{SchemaVersion: EventSchemaVersion, Sequence: sequence, ID: id, Type: eventType, AggregateID: aggregateID, OccurredAt: now.UTC(), Data: payload}, nil
}

func (e Envelope) Validate(previous uint64) error {
	if e.SchemaVersion != EventSchemaVersion {
		return fmt.Errorf("event %s schema version %d is unsupported", e.ID, e.SchemaVersion)
	}
	if e.Sequence != previous+1 {
		return fmt.Errorf("event %s sequence %d does not follow %d", e.ID, e.Sequence, previous)
	}
	if e.Type == "" || e.AggregateID == "" || len(e.Data) == 0 || !json.Valid(e.Data) {
		return fmt.Errorf("event %s is incomplete", e.ID)
	}
	return nil
}
