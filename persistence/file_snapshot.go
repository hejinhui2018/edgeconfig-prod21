package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

func (s *FileStore) Save(ctx context.Context, snapshot Snapshot) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := snapshot.Normalize(); err != nil {
		return err
	}
	last, err := s.LastSequence(ctx)
	if err != nil {
		return err
	}
	if snapshot.LastSequence > last {
		return fmt.Errorf("snapshot sequence %d exceeds event log sequence %d", snapshot.LastSequence, last)
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	data = append(data, '\n')
	return atomicWrite(s.snapshotPath, data, 0o640)
}

func (s *FileStore) Load(ctx context.Context) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	data, err := os.ReadFile(s.snapshotPath)
	if errors.Is(err, os.ErrNotExist) {
		return EmptySnapshot(), nil
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("read snapshot: %w", err)
	}
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}
	if err := snapshot.Normalize(); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}
