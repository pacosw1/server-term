package agent

import (
	"context"
	"github.com/franciscosainzwilliams/server-term/internal/collector"
	"github.com/franciscosainzwilliams/server-term/internal/config"
	"github.com/franciscosainzwilliams/server-term/internal/metrics"
	"log/slog"
	"time"
)

type Runner struct {
	Server              *Server
	Interval, Retention time.Duration
	Log                 *slog.Logger
	collector           collector.Collector
	previous            *metrics.Sample
	lastPersist         time.Time
}

func NewRunner(server *Server, interval, retention time.Duration, log *slog.Logger) *Runner {
	timeout := 2 * interval
	if timeout < 3*time.Second {
		timeout = 3 * time.Second
	}
	return &Runner{Server: server, Interval: interval, Retention: retention, Log: log, collector: collector.Collector{SSH: config.SSHConfig{CommandTimeout: timeout}}}
}
func (r *Runner) Run(ctx context.Context) error {
	if r.Interval < 250*time.Millisecond {
		r.Interval = time.Second
	}
	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()
	prune := time.NewTicker(time.Hour)
	defer prune.Stop()
	r.sample(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			r.sample(ctx)
		case <-prune.C:
			if err := r.Server.Store.Compact(ctx, time.Now()); err != nil {
				r.Log.Error("prune history", "error", err)
			}
		}
	}
}
func (r *Runner) sample(ctx context.Context) {
	s := r.collector.Collect(ctx, config.Server{Name: r.Server.NodeID, Address: "localhost", Transport: "local"})
	metrics.Derive(r.previous, &s)
	if s.Online {
		copy := s
		r.previous = &copy
	}
	persist := time.Since(r.lastPersist) >= time.Second
	if persist {
		r.lastPersist = time.Now()
	}
	if err := r.Server.Publish(ctx, s, persist); err != nil {
		r.Log.Error("store sample", "error", err)
	}
}
