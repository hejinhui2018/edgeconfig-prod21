package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"edgeconfig/agent"
	"edgeconfig/api"
	"edgeconfig/persistence"
	"edgeconfig/recovery"
	"edgeconfig/rollout"
)

func TestHTTPWorkflowAndIdempotentSiteCreation(t *testing.T) {
	store := persistence.NewMemoryStore()
	engine, _ := rollout.NewEngine(store, persistence.EmptySnapshot(), time.Now)
	agents, _ := agent.NewService(engine)
	server := httptest.NewServer(api.NewServer(engine, agents, recovery.Report{}, nil).Handler())
	defer server.Close()
	client := server.Client()
	post := func(path, key, body string) *http.Response {
		t.Helper()
		request, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+path, bytes.NewBufferString(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", key)
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	site := `{"id":"site-http","name":"HTTP Site"}`
	response := post("/v1/sites", "create-site", site)
	assertStatus(t, response, 201)
	response = post("/v1/sites", "create-site", site)
	assertStatus(t, response, 201)
	if response.Header.Get("Idempotency-Replayed") != "true" {
		t.Fatal("expected replay header")
	}
	assertStatus(t, post("/v1/devices", "create-device", `{"id":"device-http","site_id":"site-http","name":"Agent","capabilities":["json-v1"]}`), 201)
	assertStatus(t, post("/v1/configurations", "create-config", `{"id":"config-http","site_id":"site-http","version":"1.0.0","schema_version":"v1","parameters":{"port":8080},"validation":{"valid":true},"summary":"HTTP flow","required_capabilities":["json-v1"]}`), 201)
	assertStatus(t, post("/v1/rollouts", "create-rollout", `{"id":"rollout-http","site_id":"site-http","config_id":"config-http","device_ids":["device-http"],"batch_size":1,"max_attempts":2,"lease_seconds":30,"failure_strategy":"fail_fast"}`), 201)
	assertStatus(t, post("/v1/admin/dispatch", "dispatch", `{}`), 200)
	response, err := client.Get(server.URL + "/v1/agents/device-http/assignments/current")
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(t, response, 200)
	var assignment map[string]any
	if err = json.NewDecoder(response.Body).Decode(&assignment); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if assignment["lease_token"] == "" {
		t.Fatal("missing lease token")
	}
}

func assertStatus(t *testing.T, response *http.Response, want int) {
	t.Helper()
	if response.StatusCode != want {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf("status=%d want=%d body=%s", response.StatusCode, want, body)
	}
}
