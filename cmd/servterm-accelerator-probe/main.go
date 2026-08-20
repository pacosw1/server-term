package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/franciscosainzwilliams/server-term/internal/acceleratorprobe"
)

func main() {
	output := flag.String("output", "/run/servterm/accelerators.tsv", "sanitized output path")
	interval := flag.Duration("interval", time.Second, "measurement window and refresh interval")
	flag.Parse()
	stop := make(chan struct{})
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-signals; close(stop) }()
	if err := acceleratorprobe.Run(*output, *interval, nil, stop); err != nil {
		slog.Error("accelerator probe stopped", "error", err)
		os.Exit(1)
	}
}
