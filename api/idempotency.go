package api

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type cachedResponse struct {
	Hash      [32]byte
	Status    int
	Header    http.Header
	Body      []byte
	ExpiresAt time.Time
}
type IdempotencyStore struct {
	mu      sync.Mutex
	entries map[string]cachedResponse
	ttl     time.Duration
	now     func() time.Time
}

func NewIdempotencyStore(ttl time.Duration) *IdempotencyStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &IdempotencyStore{entries: make(map[string]cachedResponse), ttl: ttl, now: time.Now}
}

func (s *IdempotencyStore) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			writeError(w, r, http.StatusBadRequest, "idempotency_key_required", fmt.Errorf("Idempotency-Key header is required"))
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody+1))
		if err != nil {
			writeError(w, r, 400, "invalid_body", err)
			return
		}
		if len(body) > maxRequestBody {
			writeError(w, r, 413, "body_too_large", fmt.Errorf("request body exceeds %d bytes", maxRequestBody))
			return
		}
		hash := sha256.Sum256(append([]byte(r.Method+" "+r.URL.Path+"\n"), body...))
		s.mu.Lock()
		entry, found := s.entries[key]
		if found && entry.ExpiresAt.Before(s.now()) {
			delete(s.entries, key)
			found = false
		}
		s.mu.Unlock()
		if found {
			if entry.Hash != hash {
				writeError(w, r, http.StatusConflict, "idempotency_conflict", fmt.Errorf("idempotency key was already used for a different request"))
				return
			}
			copyHeader(w.Header(), entry.Header)
			w.Header().Set("Idempotency-Replayed", "true")
			w.WriteHeader(entry.Status)
			_, _ = w.Write(entry.Body)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		recorder := newResponseRecorder(w)
		next.ServeHTTP(recorder, r)
		if recorder.status < 500 {
			s.mu.Lock()
			s.entries[key] = cachedResponse{Hash: hash, Status: recorder.status, Header: recorder.header.Clone(), Body: append([]byte(nil), recorder.body.Bytes()...), ExpiresAt: s.now().Add(s.ttl)}
			s.mu.Unlock()
		}
	})
}

type responseRecorder struct {
	target http.ResponseWriter
	header http.Header
	body   bytes.Buffer
	status int
}

func newResponseRecorder(target http.ResponseWriter) *responseRecorder {
	return &responseRecorder{target: target, header: make(http.Header), status: http.StatusOK}
}
func (r *responseRecorder) Header() http.Header { return r.header }
func (r *responseRecorder) WriteHeader(status int) {
	if r.body.Len() > 0 {
		return
	}
	r.status = status
	copyHeader(r.target.Header(), r.header)
	r.target.WriteHeader(status)
}
func (r *responseRecorder) Write(data []byte) (int, error) {
	r.body.Write(data)
	return r.target.Write(data)
}
func copyHeader(destination, source http.Header) {
	for key, values := range source {
		destination[key] = append([]string(nil), values...)
	}
}
