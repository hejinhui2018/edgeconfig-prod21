package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Warnings []string `json:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty"`
}

type Configuration struct {
	ID                   ConfigID         `json:"id"`
	SiteID               SiteID           `json:"site_id"`
	Version              string           `json:"version"`
	SchemaVersion        string           `json:"schema_version"`
	Parameters           json.RawMessage  `json:"parameters"`
	Validation           ValidationResult `json:"validation"`
	Summary              string           `json:"summary"`
	RequiredCapabilities []string         `json:"required_capabilities,omitempty"`
	CreatedAt            time.Time        `json:"created_at"`
}

func NewConfiguration(value Configuration, now time.Time) (Configuration, error) {
	if err := ValidateID("configuration", string(value.ID)); err != nil {
		return Configuration{}, err
	}
	if err := ValidateID("site", string(value.SiteID)); err != nil {
		return Configuration{}, err
	}
	value.Version = strings.TrimSpace(value.Version)
	value.SchemaVersion = strings.TrimSpace(value.SchemaVersion)
	value.Summary = strings.TrimSpace(value.Summary)
	if value.Version == "" || value.SchemaVersion == "" {
		return Configuration{}, fmt.Errorf("configuration %s version and schema_version are required", value.ID)
	}
	if !value.Validation.Valid || len(value.Validation.Errors) > 0 {
		return Configuration{}, fmt.Errorf("configuration %s did not pass parameter validation: %v", value.ID, value.Validation.Errors)
	}
	if len(value.Parameters) == 0 || !json.Valid(value.Parameters) {
		return Configuration{}, fmt.Errorf("configuration %s parameters must be valid JSON", value.ID)
	}
	if len(value.Summary) > 500 {
		return Configuration{}, fmt.Errorf("configuration %s summary exceeds 500 characters", value.ID)
	}
	value.RequiredCapabilities = normalizeStrings(value.RequiredCapabilities)
	value.CreatedAt = now.UTC()
	return value, nil
}
