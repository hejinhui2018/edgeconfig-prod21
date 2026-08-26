package api_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"edgeconfig/agent"
	"edgeconfig/api"
	"edgeconfig/domain"
	"edgeconfig/persistence"
	"edgeconfig/recovery"
	"edgeconfig/rollout"
)

type rolloutGateStore struct {
	*persistence.MemoryStore
	mu      sync.Mutex
	armed   bool
	entered chan struct{}
	release chan struct{}
}

func newRolloutGateStore() *rolloutGateStore {
	return &rolloutGateStore{MemoryStore: persistence.NewMemoryStore()}
}

func (s *rolloutGateStore) arm() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.armed = true
	s.entered = make(chan struct{})
	s.release = make(chan struct{})
}

func (s *rolloutGateStore) Append(ctx context.Context, expected uint64, events []domain.Envelope) error {
	s.mu.Lock()
	block := s.armed
	if block {
		s.armed = false
		close(s.entered)
	}
	release := s.release
	s.mu.Unlock()
	if block {
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return s.MemoryStore.Append(ctx, expected, events)
}

func (s *rolloutGateStore) open() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.release:
	default:
		close(s.release)
	}
}

func TestConcurrentRolloutSubmissionReturnsOneBusinessResult(t *testing.T) {
	store := newRolloutGateStore()
	engine, err := rollout.NewEngine(store, persistence.EmptySnapshot(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err = engine.CreateSite(ctx, "site-concurrent", "Concurrent Site", nil); err != nil {
		t.Fatal(err)
	}
	if _, err = engine.RegisterDevice(ctx, domain.Device{ID: "device-concurrent", SiteID: "site-concurrent", Name: "Agent", Capabilities: []string{"json-v1"}}); err != nil {
		t.Fatal(err)
	}
	if _, err = engine.CreateConfiguration(ctx, domain.Configuration{ID: "config-concurrent", SiteID: "site-concurrent", Version: "1.0.0", SchemaVersion: "v1", Parameters: []byte(`{"enabled":true}`), Validation: domain.ValidationResult{Valid: true}, Summary: "Concurrent rollout", RequiredCapabilities: []string{"json-v1"}}); err != nil {
		t.Fatal(err)
	}
	agents, err := agent.NewService(engine)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(api.NewServer(engine, agents, recovery.Report{}, nil).Handler())
	defer server.Close()

	body := []byte(`{"id":"rollout-concurrent","site_id":"site-concurrent","config_id":"config-concurrent","device_ids":["device-concurrent"]}`)
	post := func() *http.Response {
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/v1/rollouts", bytes.NewReader(body))
		if requestErr != nil {
			t.Error(requestErr)
			return nil
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "rollout-concurrent-request")
		response, requestErr := server.Client().Do(request)
		if requestErr != nil {
			t.Error(requestErr)
			return nil
		}
		return response
	}

	store.arm()
	first := make(chan *http.Response, 1)
	go func() { first <- post() }()
	select {
	case <-store.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("rollout persistence did not reach the concurrency gate")
	}

	second := make(chan *http.Response, 1)
	go func() { second <- post() }()
	select {
	case response := <-second:
		if response != nil {
			response.Body.Close()
		}
		store.open()
		t.Fatal("duplicate submission completed before the first result was committed")
	case <-time.After(250 * time.Millisecond):
	}
	store.open()

	firstResponse := <-first
	secondResponse := <-second
	if firstResponse == nil || secondResponse == nil {
		t.Fatal("duplicate submission did not produce two responses")
	}
	defer firstResponse.Body.Close()
	defer secondResponse.Body.Close()
	if firstResponse.StatusCode != http.StatusCreated || secondResponse.StatusCode != http.StatusCreated {
		t.Fatalf("statuses=%d,%d want two created responses", firstResponse.StatusCode, secondResponse.StatusCode)
	}
	firstBody, err := io.ReadAll(firstResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	secondBody, err := io.ReadAll(secondResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBody, secondBody) {
		t.Fatalf("duplicate submission bodies differ: first=%s second=%s", firstBody, secondBody)
	}
	if secondResponse.Header.Get("Idempotency-Replayed") != "true" {
		t.Fatal("duplicate submission did not replay the committed result")
	}
	rollouts := engine.ListRollouts("site-concurrent")
	if len(rollouts) != 1 {
		t.Fatalf("rollout count=%d want one", len(rollouts))
	}
}
