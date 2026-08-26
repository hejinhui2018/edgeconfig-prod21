package api

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"edgeconfig/agent"
	"edgeconfig/domain"
	"edgeconfig/recovery"
	"edgeconfig/rollout"
)

type Server struct {
	engine      *rollout.Engine
	agents      *agent.Service
	recovery    recovery.Report
	logger      *slog.Logger
	idempotency *IdempotencyStore
	handler     http.Handler
}

func NewServer(engine *rollout.Engine, agents *agent.Service, report recovery.Report, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	server := &Server{engine: engine, agents: agents, recovery: report, logger: logger, idempotency: NewIdempotencyStore(24 * time.Hour)}
	server.handler = server.routes()
	return server
}

func (s *Server) Handler() http.Handler { return s.handler }

type requestIDKey struct{}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := domain.NewID("req", started)
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, requestID))
		w.Header().Set("X-Request-ID", requestID)
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("http panic", "panic", recovered, "stack", string(debug.Stack()))
				writeError(w, r, 500, "internal_error", context.Canceled)
			}
			s.logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds(), "request_id", requestID)
		}()
		next.ServeHTTP(w, r)
	})
}
