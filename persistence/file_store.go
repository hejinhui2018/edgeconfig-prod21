package persistence

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"edgeconfig/domain"
)

type FileStore struct {
	dir          string
	eventsPath   string
	snapshotPath string
	mu           sync.Mutex
	lastSequence uint64
}

func OpenFileStore(dir string) (*FileStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("data directory is required")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	store := &FileStore{dir: dir, eventsPath: filepath.Join(dir, "events.jsonl"), snapshotPath: filepath.Join(dir, "snapshot.json")}
	file, err := os.OpenFile(store.eventsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("create event log: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close event log after creation: %w", err)
	}
	events, report, err := store.readAfterUnlocked(context.Background(), 0)
	if err != nil {
		return nil, err
	}
	if len(events) > 0 {
		store.lastSequence = events[len(events)-1].Sequence
	} else {
		store.lastSequence = report.LastSequence
	}
	return store, nil
}

func (s *FileStore) Append(ctx context.Context, expected uint64, events []domain.Envelope) error {
	if len(events) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if expected != s.lastSequence {
		return fmt.Errorf("%w: expected %d, actual %d", ErrConflict, expected, s.lastSequence)
	}
	buffer := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(buffer)
	previous := expected
	for _, event := range events {
		if err := event.Validate(previous); err != nil {
			return err
		}
		if err := encoder.Encode(event); err != nil {
			return fmt.Errorf("encode event %s: %w", event.ID, err)
		}
		previous = event.Sequence
	}
	file, err := os.OpenFile(s.eventsPath, os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("open event log: %w", err)
	}
	if _, err = file.Write(buffer.Bytes()); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("durably append events: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close event log: %w", closeErr)
	}
	s.lastSequence = previous
	return nil
}

func (s *FileStore) ReadAfter(ctx context.Context, sequence uint64) ([]domain.Envelope, ReadReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readAfterUnlocked(ctx, sequence)
}

func (s *FileStore) readAfterUnlocked(ctx context.Context, sequence uint64) ([]domain.Envelope, ReadReport, error) {
	data, err := os.ReadFile(s.eventsPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ReadReport{}, nil
	}
	if err != nil {
		return nil, ReadReport{}, fmt.Errorf("read event log: %w", err)
	}
	report := ReadReport{}
	events := make([]domain.Envelope, 0)
	reader := bufio.NewReader(bytes.NewReader(data))
	var previous uint64
	var consumed int64
	for {
		if err := ctx.Err(); err != nil {
			return nil, report, err
		}
		line, readErr := reader.ReadBytes('\n')
		consumed += int64(len(line))
		if len(bytes.TrimSpace(line)) == 0 && readErr == io.EOF {
			break
		}
		if readErr == io.EOF && len(line) > 0 {
			var tail domain.Envelope
			if err := json.Unmarshal(line, &tail); err != nil {
				report.IgnoredTailBytes = int64(len(data)) - (consumed - int64(len(line)))
				break
			}
		}
		var event domain.Envelope
		if err := json.Unmarshal(bytes.TrimSpace(line), &event); err != nil {
			return nil, report, fmt.Errorf("decode event record ending at byte %d: %w", consumed, err)
		}
		if err := event.Validate(previous); err != nil {
			return nil, report, err
		}
		previous = event.Sequence
		report.Records++
		report.LastSequence = event.Sequence
		if event.Sequence > sequence {
			events = append(events, event)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, report, fmt.Errorf("scan event log: %w", readErr)
		}
	}
	return events, report, nil
}

func (s *FileStore) LastSequence(ctx context.Context) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return s.lastSequence, nil
}
