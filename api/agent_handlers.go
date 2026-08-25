package api

import (
	"net/http"

	"edgeconfig/domain"
)

func (s *Server) pullAssignment(w http.ResponseWriter, r *http.Request) {
	value, err := s.agents.Pull(r.Context(), domain.DeviceID(r.PathValue("device_id")))
	if err != nil {
		s.engineError(w, r, err)
		return
	}
	if value == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, 200, value)
}

func (s *Server) reportEvent(w http.ResponseWriter, r *http.Request) {
	var event domain.DeviceEvent
	if err := decodeJSON(w, r, &event); err != nil {
		writeError(w, r, 400, "invalid_request", err)
		return
	}
	pathDevice := domain.DeviceID(r.PathValue("device_id"))
	if event.DeviceID == "" {
		event.DeviceID = pathDevice
	}
	if event.DeviceID != pathDevice {
		writeError(w, r, 400, "device_mismatch", errDeviceMismatch)
		return
	}
	result, err := s.agents.Report(r.Context(), event, r.Header.Get("X-Lease-Token"))
	if err != nil {
		s.engineError(w, r, err)
		return
	}
	writeJSON(w, 200, result)
}

var errDeviceMismatch = &requestError{"event device_id does not match URL device"}

type requestError struct{ message string }

func (e *requestError) Error() string { return e.message }
