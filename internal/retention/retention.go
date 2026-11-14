// Package retention deletes handled reports once their retention window ends.
package retention

import (
	"context"
	"log/slog"
	"time"

	"github.com/janrk/cairn/internal/store"
)

// DefaultInterval is how often Run sweeps when Interval is zero.
const DefaultInterval = 10 * time.Minute

// Sweeper deletes reports handled more than Retention ago.
type Sweeper struct {
	Store     *store.Store
	Retention time.Duration
	Interval  time.Duration
	Now       func() time.Time
	Log       *slog.Logger
}

// Sweep runs one deletion pass and logs the number deleted.
func (s *Sweeper) Sweep(ctx context.Context) (int64, error) {
	n, err := s.Store.DeleteHandledBefore(ctx, s.Now().Add(-s.Retention))
	if err != nil {
		s.Log.Error("retention sweep failed", "err", err)
		return 0, err
	}
	s.Log.Info("retention sweep", "deleted", n)
	return n, nil
}

// Run sweeps immediately and then every Interval until ctx is done.
func (s *Sweeper) Run(ctx context.Context) {
	interval := s.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		_, _ = s.Sweep(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
