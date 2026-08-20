package main

import (
	"flag"
	"github.com/franciscosainzwilliams/server-term/internal/runnerprobe"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	output := flag.String("output", "/run/servterm/runner-jobs.jsonl", "sanitized output path")
	interval := flag.Duration("interval", time.Second, "probe interval")
	flag.Parse()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	stop := make(chan struct{})
	go func() { <-signals; close(stop) }()
	if err := runnerprobe.Run(*output, *interval, stop); err != nil {
		slog.Error("runner probe stopped", "error", err)
		os.Exit(1)
	}
}
