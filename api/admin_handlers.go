package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"edgeconfig/domain"
	"edgeconfig/rollout"
)

func (s *Server) createSite(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID     domain.SiteID     `json:"id"`
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, r, 400, "invalid_request", err)
		return
	}
	value, err := s.engine.CreateSite(r.Context(), input.ID, input.Name, input.Labels)
	if err != nil {
		s.engineError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func (s *Server) createDevice(w http.ResponseWriter, r *http.Request) {
	var input domain.Device
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, r, 400, "invalid_request", err)
		return
	}
	value, err := s.engine.RegisterDevice(r.Context(), input)
	if err != nil {
		s.engineError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

type configurationInput struct {
	ID                   domain.ConfigID         `json:"id"`
	SiteID               domain.SiteID           `json:"site_id"`
	Version              string                  `json:"version"`
	SchemaVersion        string                  `json:"schema_version"`
	Parameters           json.RawMessage         `json:"parameters"`
	Validation           domain.ValidationResult `json:"validation"`
	Summary              string                  `json:"summary"`
	RequiredCapabilities []string                `json:"required_capabilities"`
}

func (s *Server) createConfiguration(w http.ResponseWriter, r *http.Request) {
	var input configurationInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, r, 400, "invalid_request", err)
		return
	}
	value, err := s.engine.CreateConfiguration(r.Context(), domain.Configuration{ID: input.ID, SiteID: input.SiteID, Version: input.Version, SchemaVersion: input.SchemaVersion, Parameters: input.Parameters, Validation: input.Validation, Summary: input.Summary, RequiredCapabilities: input.RequiredCapabilities})
	if err != nil {
		s.engineError(w, r, err)
		return
	}
	writeJSON(w, 201, value)
}

type rolloutInput struct {
	ID              domain.RolloutID  `json:"id"`
	SiteID          domain.SiteID     `json:"site_id"`
	ConfigID        domain.ConfigID   `json:"config_id"`
	DeviceIDs       []domain.DeviceID `json:"device_ids"`
	Selector        map[string]string `json:"selector"`
	BatchSize       int               `json:"batch_size"`
	MaxAttempts     int               `json:"max_attempts"`
	LeaseSeconds    int               `json:"lease_seconds"`
	RetrySeconds    int               `json:"retry_seconds"`
	FailureStrategy string            `json:"failure_strategy"`
}

func (s *Server) createRollout(w http.ResponseWriter, r *http.Request) {
	var input rolloutInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, r, 400, "invalid_request", err)
		return
	}
	policy := rollout.DefaultPolicy()
	if input.BatchSize != 0 {
		policy.BatchSize = input.BatchSize
	}
	if input.MaxAttempts != 0 {
		policy.MaxAttempts = input.MaxAttempts
	}
	if input.LeaseSeconds != 0 {
		policy.LeaseDuration = time.Duration(input.LeaseSeconds) * time.Second
	}
	if input.RetrySeconds != 0 {
		policy.RetryBackoff = time.Duration(input.RetrySeconds) * time.Second
	}
	if input.FailureStrategy != "" {
		policy.FailureStrategy = input.FailureStrategy
	}
	value, err := s.engine.CreateRollout(r.Context(), rollout.CreateRequest{ID: input.ID, SiteID: input.SiteID, ConfigID: input.ConfigID, DeviceIDs: input.DeviceIDs, Selector: input.Selector, Policy: policy})
	if err != nil {
		s.engineError(w, r, err)
		return
	}
	writeJSON(w, 201, value)
}

func (s *Server) listRollouts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"rollouts": s.engine.ListRollouts(domain.SiteID(r.URL.Query().Get("site_id")))})
}
func (s *Server) getRollout(w http.ResponseWriter, r *http.Request) {
	value, err := s.engine.GetRollout(domain.RolloutID(r.PathValue("rollout_id")))
	if err != nil {
		s.engineError(w, r, err)
		return
	}
	writeJSON(w, 200, value)
}
func (s *Server) dispatch(w http.ResponseWriter, r *http.Request) {
	count, err := s.engine.Dispatch(r.Context())
	if err != nil {
		s.engineError(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]int{"assignments_created": count})
}

func (s *Server) engineError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, rollout.ErrNotFound):
		writeError(w, r, 404, "not_found", err)
	case errors.Is(err, rollout.ErrAlreadyExists):
		writeError(w, r, 409, "already_exists", err)
	case errors.Is(err, rollout.ErrOutOfOrder):
		writeError(w, r, 409, "out_of_order", err)
	default:
		writeError(w, r, 422, "invalid_state", err)
	}
}
