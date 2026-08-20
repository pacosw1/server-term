package agent_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/franciscosainzwilliams/server-term/internal/agent"
	"github.com/franciscosainzwilliams/server-term/internal/agentclient"
	"github.com/franciscosainzwilliams/server-term/internal/metrics"
)

func TestPublishPersistsAndStreams(t *testing.T) {
	store, err := agent.OpenStore(t.TempDir() + "/metrics.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := agent.NewServer("node-1", "secret", store, nil)
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := agentclient.Connect(ctx, httpServer.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	want := metrics.Sample{At: time.Now().UTC().Truncate(time.Millisecond), Online: true, Hostname: "box", Cores: 32, MemTotal: 64 << 30}
	if err := server.Publish(ctx, want, true); err != nil {
		t.Fatal(err)
	}
	got, err := stream.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.NodeID != "node-1" || got.Sample.Cores != 32 || got.Sample.Hostname != "box" {
		t.Fatalf("unexpected stream payload: %+v", got)
	}

	history, err := agentclient.History(ctx, httpServer.URL, "secret", time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].MemTotal != want.MemTotal {
		t.Fatalf("unexpected history: %+v", history)
	}
}

func TestAuthenticationAndListenSafety(t *testing.T) {
	store, err := agent.OpenStore(t.TempDir() + "/metrics.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := httptest.NewServer(agent.NewServer("node", "secret", store, nil).Handler())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := agentclient.History(ctx, server.URL, "wrong", time.Hour, 10); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401, got %v", err)
	}
	if err := agent.ListenAddressSafe("100.64.0.10:7843", ""); err == nil {
		t.Fatal("non-loopback listener accepted without token")
	}
	if err := agent.ListenAddressSafe("127.0.0.1:7843", ""); err != nil {
		t.Fatal(err)
	}
}
