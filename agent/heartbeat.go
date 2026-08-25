package agent

import (
	"context"
	"fmt"
	"time"

	"edgeconfig/rollout"
)

type LeaseReaper struct {
	Engine   *rollout.Engine
	Interval time.Duration
	OnError  func(error)
}

func (r LeaseReaper) Run(ctx context.Context) error {
	if r.Engine == nil {
		return fmt.Errorf("lease reaper engine is required")
	}
	if r.Interval <= 0 {
		r.Interval = time.Second
	}
	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := r.Engine.ReapExpiredLeases(ctx); err != nil && ctx.Err() == nil && r.OnError != nil {
				r.OnError(err)
			}
		}
	}
}
