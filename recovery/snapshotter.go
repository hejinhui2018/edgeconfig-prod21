package recovery

import (
	"context"
	"fmt"
	"time"

	"edgeconfig/rollout"
)

type Snapshotter struct {
	Engine   *rollout.Engine
	Interval time.Duration
	OnError  func(error)
}

func (s Snapshotter) Run(ctx context.Context) error {
	if s.Engine == nil {
		return fmt.Errorf("snapshotter engine is required")
	}
	if s.Interval <= 0 {
		s.Interval = 30 * time.Second
	}
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			finalCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.Engine.SaveSnapshot(finalCtx); err != nil {
				return fmt.Errorf("save final snapshot: %w", err)
			}
			return ctx.Err()
		case <-ticker.C:
			if err := s.Engine.SaveSnapshot(ctx); err != nil && s.OnError != nil {
				s.OnError(err)
			}
		}
	}
}
