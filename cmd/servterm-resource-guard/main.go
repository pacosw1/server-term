package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/franciscosainzwilliams/server-term/internal/resourceguard"
)

func main() {
	cgroup := flag.String("cgroup", "/sys/fs/cgroup/ci.slice/ci-runners.slice", "runner slice cgroup path")
	interval := flag.Duration("interval", 5*time.Second, "sampling interval")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := resourceguard.Run(ctx, *cgroup, *interval); err != nil {
		slog.Error("resource guard stopped", "error", err)
		os.Exit(1)
	}
}
