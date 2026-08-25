package domain

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

type Device struct {
	ID             DeviceID          `json:"id"`
	SiteID         SiteID            `json:"site_id"`
	Name           string            `json:"name"`
	Capabilities   []string          `json:"capabilities,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	CurrentVersion string            `json:"current_version,omitempty"`
	LastHeartbeat  time.Time         `json:"last_heartbeat,omitempty"`
	RegisteredAt   time.Time         `json:"registered_at"`
}

func NewDevice(id DeviceID, siteID SiteID, name string, capabilities []string, labels map[string]string, now time.Time) (Device, error) {
	if err := ValidateID("device", string(id)); err != nil {
		return Device{}, err
	}
	if err := ValidateID("site", string(siteID)); err != nil {
		return Device{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 120 {
		return Device{}, fmt.Errorf("device %s name must contain 1 to 120 characters", id)
	}
	capabilities = normalizeStrings(capabilities)
	return Device{ID: id, SiteID: siteID, Name: name, Capabilities: capabilities, Labels: cloneMap(labels), RegisteredAt: now.UTC()}, nil
}

func (d Device) Supports(required []string) bool {
	for _, capability := range required {
		if !slices.Contains(d.Capabilities, capability) {
			return false
		}
	}
	return true
}

func normalizeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}
