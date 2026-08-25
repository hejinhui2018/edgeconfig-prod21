package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type SiteID string
type DeviceID string
type ConfigID string
type RolloutID string
type AssignmentID string
type EventID string

func NewID(prefix string, now time.Time) string {
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, now.UTC().UnixNano())
	}
	return fmt.Sprintf("%s_%d_%s", prefix, now.UTC().UnixMilli(), hex.EncodeToString(suffix[:]))
}

func ValidateID(kind, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s id is required", kind)
	}
	if len(value) > 128 {
		return fmt.Errorf("%s id exceeds 128 characters", kind)
	}
	for _, r := range value {
		if !(r == '-' || r == '_' || r == '.' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return fmt.Errorf("%s id %q contains unsupported character %q", kind, value, r)
		}
	}
	return nil
}
