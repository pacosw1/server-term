package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/franciscosainzwilliams/server-term/internal/agent"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var version = "dev"

func main() {
	if err := run(); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("agent stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	var listen, db, node, tokenFile string
	var interval, retention time.Duration
	flag.StringVar(&listen, "listen", "127.0.0.1:7843", "HTTP/WebSocket listen address")
	flag.StringVar(&db, "db", "/var/lib/servterm/metrics.db", "SQLite database path")
	flag.StringVar(&node, "node", hostname(), "stable node name")
	flag.StringVar(&tokenFile, "token-file", "", "read bearer token from a protected file")
	flag.DurationVar(&interval, "interval", time.Second, "live stream sample interval")
	flag.DurationVar(&retention, "retention", 30*24*time.Hour, "history retention")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println("servterm-agent " + version)
		return nil
	}
	token, err := agent.LoadToken(tokenFile)
	if err != nil {
		return err
	}
	if err := agent.ListenAddressSafe(listen, token); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(db), 0750); err != nil {
		return err
	}
	store, err := agent.OpenStore(db)
	if err != nil {
		return err
	}
	defer store.Close()
	log := slog.Default()
	server := agent.NewServer(node, token, store, log)
	runner := agent.NewRunner(server, interval, retention, log)
	httpServer := &http.Server{Addr: listen, Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdown)
	}()
	go func() {
		if err := runner.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("sampler stopped", "error", err)
		}
	}()
	log.Info("servterm agent listening", "address", listen, "node", node, "interval", interval, "retention", retention)
	err = httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}
	return h
}
