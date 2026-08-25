package persistence

import (
	"fmt"
	"time"

	"edgeconfig/domain"
)

const SnapshotSchemaVersion = 1

type Snapshot struct {
	SchemaVersion   int                                       `json:"schema_version"`
	LastSequence    uint64                                    `json:"last_sequence"`
	CreatedAt       time.Time                                 `json:"created_at"`
	Sites           map[domain.SiteID]domain.Site             `json:"sites"`
	Devices         map[domain.DeviceID]domain.Device         `json:"devices"`
	Configurations  map[domain.ConfigID]domain.Configuration  `json:"configurations"`
	Rollouts        map[domain.RolloutID]*domain.Rollout      `json:"rollouts"`
	Assignments     map[domain.AssignmentID]domain.Assignment `json:"assignments"`
	ProcessedEvents map[domain.EventID]uint64                 `json:"processed_events"`
	DeviceSequences map[domain.DeviceID]uint64                `json:"device_sequences"`
}

func EmptySnapshot() Snapshot {
	return Snapshot{
		SchemaVersion:   SnapshotSchemaVersion,
		Sites:           make(map[domain.SiteID]domain.Site),
		Devices:         make(map[domain.DeviceID]domain.Device),
		Configurations:  make(map[domain.ConfigID]domain.Configuration),
		Rollouts:        make(map[domain.RolloutID]*domain.Rollout),
		Assignments:     make(map[domain.AssignmentID]domain.Assignment),
		ProcessedEvents: make(map[domain.EventID]uint64),
		DeviceSequences: make(map[domain.DeviceID]uint64),
	}
}

func (s *Snapshot) Normalize() error {
	if s.SchemaVersion == 0 && s.LastSequence == 0 {
		*s = EmptySnapshot()
		return nil
	}
	if s.SchemaVersion != SnapshotSchemaVersion {
		return fmt.Errorf("snapshot schema version %d is unsupported", s.SchemaVersion)
	}
	if s.Sites == nil {
		s.Sites = make(map[domain.SiteID]domain.Site)
	}
	if s.Devices == nil {
		s.Devices = make(map[domain.DeviceID]domain.Device)
	}
	if s.Configurations == nil {
		s.Configurations = make(map[domain.ConfigID]domain.Configuration)
	}
	if s.Rollouts == nil {
		s.Rollouts = make(map[domain.RolloutID]*domain.Rollout)
	}
	if s.Assignments == nil {
		s.Assignments = make(map[domain.AssignmentID]domain.Assignment)
	}
	if s.ProcessedEvents == nil {
		s.ProcessedEvents = make(map[domain.EventID]uint64)
	}
	if s.DeviceSequences == nil {
		s.DeviceSequences = make(map[domain.DeviceID]uint64)
	}
	return nil
}
