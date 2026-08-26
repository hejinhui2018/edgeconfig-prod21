package domain

import (
	"fmt"
	"strings"
	"time"
)

type Site struct {
	ID        SiteID            `json:"id"`
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

func NewSite(id SiteID, name string, labels map[string]string, now time.Time) (Site, error) {
	if err := ValidateID("site", string(id)); err != nil {
		return Site{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 120 {
		return Site{}, fmt.Errorf("site %s name must contain 1 to 120 characters", id)
	}
	return Site{ID: id, Name: name, Labels: cloneMap(labels), CreatedAt: now.UTC()}, nil
}

func cloneMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	copy := make(map[string]string, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}
