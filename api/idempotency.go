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
	// done is closed once the in-flight handler has finished. Concurrent
	// retries with the same key wait on it so they replay the first response
	// instead of re-executing the handler.
	done chan struct{}
	// ready is true once a successful response has been recorded and is safe
	// to replay. It distinguishes "in-flight" from "complete".
	ready bool
}

type IdempotencyStore struct {
	mu      sync.Mutex
	entries map[string]*cachedResponse
	ttl     time.Duration
	now     func() time.Time
}

func NewIdempotencyStore(ttl time.Duration) *IdempotencyStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &IdempotencyStore{entries: make(map[string]*cachedResponse), ttl: ttl, now: time.Now}
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

		// Claim the key atomically under one lock: either reuse a completed
		// response, wait on an in-flight request, or register this request as
		// in-flight. This closes the TOCTOU window where two retries with the
		// same key both missed the cache and re-executed the handler, which
		// produced a success for one retry and a conflict for the other.
		// Re-claim after every wait instead of trusting a stale entry pointer:
		// a non-2xx response removes the placeholder, so a waiting retry must
		// register a fresh in-flight slot rather than re-execute concurrently.
		for {
			entry, wait, ready, conflictErr := s.claim(key, hash)
			if conflictErr != nil {
				writeError(w, r, http.StatusConflict, "idempotency_conflict", conflictErr)
				return
			}
			if ready {
				s.replay(w, entry)
				return
			}
			if wait == nil {
				// This request owns the in-flight placeholder; execute the
				// handler and record the result for future retries.
				r.Body = io.NopCloser(bytes.NewReader(body))
				recorder := newResponseRecorder(w)
				next.ServeHTTP(recorder, r)
				s.finish(key, recorder.status, recorder.header, recorder.body.Bytes())
				return
			}
			<-wait
		}
	})
}

// claim looks up the key and returns one of:
//   - a ready entry with a nil wait (replay the cached response),
//   - an in-flight entry with a non-nil wait channel (wait, then replay),
//   - a newly registered in-flight entry with a nil wait (execute the handler).
//
// A non-nil conflictErr means the key was already used for a different request body.
func (s *IdempotencyStore) claim(key string, hash [32]byte) (*cachedResponse, chan struct{}, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, found := s.entries[key]
	if found && entry.ExpiresAt.Before(s.now()) {
		delete(s.entries, key)
		found = false
	}
	if !found {
		entry = &cachedResponse{Hash: hash, done: make(chan struct{}), ExpiresAt: s.now().Add(s.ttl)}
		s.entries[key] = entry
		return entry, nil, false, nil
	}
	if entry.Hash != hash {
		return nil, nil, false, fmt.Errorf("idempotency key was already used for a different request")
	}
	if entry.ready {
		return entry, nil, true, nil
	}
	return entry, entry.done, false, nil
}

// finish records the handler response on the in-flight entry. Only successful
// (2xx) responses are cached for replay; the in-flight placeholder is removed
// for non-2xx so a later retry with the same key is not poisoned.
func (s *IdempotencyStore) finish(key string, status int, header http.Header, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, found := s.entries[key]
	if !found {
		return
	}
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		entry.Status = status
		entry.Header = header.Clone()
		entry.Body = append([]byte(nil), body...)
		entry.ready = true
	} else {
		delete(s.entries, key)
	}
	if entry.done != nil {
		close(entry.done)
		entry.done = nil
	}
}

// replay writes a cached response back to a retried request and marks it as a
// replay with the Idempotency-Replayed header.
func (s *IdempotencyStore) replay(w http.ResponseWriter, entry *cachedResponse) {
	copyHeader(w.Header(), entry.Header)
	w.Header().Set("Idempotency-Replayed", "true")
	w.WriteHeader(entry.Status)
	_, _ = w.Write(entry.Body)
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
