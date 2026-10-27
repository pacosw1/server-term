package agent

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

// desktop carries RFB bytes to this host's existing VNC server. It never
// stores a Mac password or replaces the VNC server's own authentication.
// This endpoint is opt-in and requires the existing agent bearer token.
func (s *Server) desktop(w http.ResponseWriter, r *http.Request) {
	backend, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(r.Context(), "tcp", "127.0.0.1:5900")
	if err != nil {
		http.Error(w, "Screen sharing is unavailable on this server", http.StatusServiceUnavailable)
		return
	}
	defer backend.Close()
	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer ws.CloseNow()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stream := websocket.NetConn(ctx, ws, websocket.MessageBinary)
	defer stream.Close()
	done := make(chan struct{})
	go func() { defer close(done); _, _ = io.Copy(backend, stream); cancel(); _ = backend.Close() }()
	_, _ = io.Copy(stream, backend)
	cancel()
	_ = backend.Close()
	_ = stream.Close()
	<-done
}
