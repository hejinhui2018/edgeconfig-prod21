package rollout

import (
	"fmt"

	"edgeconfig/domain"
	"edgeconfig/persistence"
)

func apply(state *persistence.Snapshot, event domain.Envelope) error {
	if event.Sequence != state.LastSequence+1 {
		return fmt.Errorf("event sequence %d does not follow state sequence %d", event.Sequence, state.LastSequence)
	}
	switch event.Type {
	case domain.EventSiteCreated:
		value, err := decode[domain.Site](event)
		if err != nil {
			return err
		}
		state.Sites[value.ID] = value
	case domain.EventDeviceRegistered:
		value, err := decode[domain.Device](event)
		if err != nil {
			return err
		}
		state.Devices[value.ID] = value
	case domain.EventConfigurationCreated:
		value, err := decode[domain.Configuration](event)
		if err != nil {
			return err
		}
		state.Configurations[value.ID] = value
	case domain.EventRolloutCreated:
		value, err := decode[domain.Rollout](event)
		if err != nil {
			return err
		}
		state.Rollouts[value.ID] = &value
	case domain.EventRolloutQueued:
		value, err := decode[rolloutQueued](event)
		if err != nil {
			return err
		}
		rollout := state.Rollouts[value.RolloutID]
		if rollout == nil {
			return fmt.Errorf("queue unknown rollout %s", value.RolloutID)
		}
		rollout.Status = domain.RolloutQueued
		rollout.UpdatedAt = event.OccurredAt
	case domain.EventAssignmentCreated:
		value, err := decode[assignmentCreated](event)
		if err != nil {
			return err
		}
		state.Assignments[value.Assignment.ID] = value.Assignment
		rollout := state.Rollouts[value.Assignment.RolloutID]
		if rollout == nil {
			return fmt.Errorf("assignment references unknown rollout")
		}
		target := rollout.Targets[value.Assignment.DeviceID]
		if target == nil {
			return fmt.Errorf("assignment references unknown target")
		}
		target.Status = domain.TargetAssigned
		target.Attempts = value.Assignment.Attempt
		target.AssignmentID = value.Assignment.ID
		target.LastEventAt = event.OccurredAt
	case domain.EventAssignmentLeased:
		value, err := decode[assignmentLeased](event)
		if err != nil {
			return err
		}
		assignment, ok := state.Assignments[value.AssignmentID]
		if !ok {
			return fmt.Errorf("lease unknown assignment %s", value.AssignmentID)
		}
		assignment.LeaseToken = value.LeaseToken
		assignment.LeaseExpiresAt = value.ExpiresAt
		state.Assignments[value.AssignmentID] = assignment
	case domain.EventDeviceProgressed:
		value, err := decode[deviceProgressed](event)
		if err != nil {
			return err
		}
		rollout := state.Rollouts[value.Event.RolloutID]
		if rollout == nil {
			return fmt.Errorf("progress unknown rollout")
		}
		target := rollout.Targets[value.Event.DeviceID]
		if target == nil {
			return fmt.Errorf("progress unknown target")
		}
		target.Status = value.Status
		target.Attempts = value.Attempts
		target.LastEventID = value.Event.ID
		target.LastEventAt = value.Event.ObservedAt
		target.LastError = value.Event.Error
		state.ProcessedEvents[value.Event.ID] = value.Event.Sequence
		state.DeviceSequences[value.Event.DeviceID] = value.Event.Sequence
		device := state.Devices[value.Event.DeviceID]
		device.LastHeartbeat = value.Event.ObservedAt
		if value.Status == domain.TargetHealthChecked {
			config := state.Configurations[rollout.ConfigID]
			device.CurrentVersion = config.Version
		}
		state.Devices[value.Event.DeviceID] = device
	case domain.EventTargetRetryScheduled:
		value, err := decode[retryScheduled](event)
		if err != nil {
			return err
		}
		rollout := state.Rollouts[value.RolloutID]
		if rollout == nil {
			return fmt.Errorf("retry unknown rollout")
		}
		target := rollout.Targets[value.DeviceID]
		if target == nil {
			return fmt.Errorf("retry unknown target")
		}
		target.Status = domain.TargetPending
		target.NextAttemptAt = value.NextAttemptAt
		target.LastError = value.LastError
		target.AssignmentID = ""
	case domain.EventRolloutStatusChanged:
		value, err := decode[rolloutStatusChanged](event)
		if err != nil {
			return err
		}
		rollout := state.Rollouts[value.RolloutID]
		if rollout == nil {
			return fmt.Errorf("status unknown rollout")
		}
		rollout.Status = value.To
		rollout.UpdatedAt = event.OccurredAt
	default:
		return fmt.Errorf("unsupported event type %q", event.Type)
	}
	state.LastSequence = event.Sequence
	return nil
}
