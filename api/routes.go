package api

import "net/http"

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /v1/recovery", s.getRecovery)
	mux.HandleFunc("POST /v1/sites", s.createSite)
	mux.HandleFunc("POST /v1/devices", s.createDevice)
	mux.HandleFunc("POST /v1/configurations", s.createConfiguration)
	mux.HandleFunc("POST /v1/rollouts", s.createRollout)
	mux.HandleFunc("GET /v1/rollouts", s.listRollouts)
	mux.HandleFunc("GET /v1/rollouts/{rollout_id}", s.getRollout)
	mux.HandleFunc("POST /v1/admin/dispatch", s.dispatch)
	mux.HandleFunc("GET /v1/agents/{device_id}/assignments/current", s.pullAssignment)
	mux.HandleFunc("POST /v1/agents/{device_id}/events", s.reportEvent)
	return s.middleware(s.idempotency.Middleware(mux))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "sequence": s.engine.LastSequence()})
}
func (s *Server) getRecovery(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, s.recovery) }
