package testutil

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"edgeconfig/domain"
	"edgeconfig/persistence"
	"edgeconfig/rollout"
)

type Fixture struct {
	Engine   *rollout.Engine
	Store    *persistence.MemoryStore
	Now      time.Time
	SiteID   domain.SiteID
	DeviceID domain.DeviceID
	ConfigID domain.ConfigID
}

func NewFixture(t testing.TB) Fixture {
	t.Helper()
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	store := persistence.NewMemoryStore()
	engine, err := rollout.NewEngine(store, persistence.EmptySnapshot(), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	fixture := Fixture{Engine: engine, Store: store, Now: now, SiteID: "site-a", DeviceID: "device-a", ConfigID: "config-a"}
	ctx := context.Background()
	if _, err = engine.CreateSite(ctx, fixture.SiteID, "Test Site", map[string]string{"region": "east"}); err != nil {
		t.Fatal(err)
	}
	if _, err = engine.RegisterDevice(ctx, domain.Device{ID: fixture.DeviceID, SiteID: fixture.SiteID, Name: "Test Device", Capabilities: []string{"json-v1"}, Labels: map[string]string{"ring": "canary"}}); err != nil {
		t.Fatal(err)
	}
	if _, err = engine.CreateConfiguration(ctx, domain.Configuration{ID: fixture.ConfigID, SiteID: fixture.SiteID, Version: "2.0.0", SchemaVersion: "v1", Parameters: json.RawMessage(`{"enabled":true}`), Validation: domain.ValidationResult{Valid: true}, Summary: "test configuration", RequiredCapabilities: []string{"json-v1"}}); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (f Fixture) CreateRollout(t testing.TB, id domain.RolloutID, policy domain.RolloutPolicy) *domain.Rollout {
	t.Helper()
	value, err := f.Engine.CreateRollout(context.Background(), rollout.CreateRequest{ID: id, SiteID: f.SiteID, ConfigID: f.ConfigID, DeviceIDs: []domain.DeviceID{f.DeviceID}, Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	return value
}
