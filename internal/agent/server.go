package agent

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/franciscosainzwilliams/server-term/internal/metrics"
)

type Server struct {
	NodeID, Token string
	Store         *Store
	Log           *slog.Logger
	mu            sync.Mutex
	clients       map[chan []byte]struct{}
	latest        metrics.WireSample
}

func NewServer(nodeID, token string, store *Store, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{NodeID: nodeID, Token: token, Store: store, Log: log, clients: map[chan []byte]struct{}{}}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/status", s.status)
	mux.HandleFunc("GET /v1/history", s.auth(s.history))
	mux.HandleFunc("GET /v1/stream", s.auth(s.stream))
	return mux
}
func (s *Server) Publish(ctx context.Context, sample metrics.Sample, persist bool) error {
	w := metrics.WireSample{Version: 1, NodeID: s.NodeID, Sample: sample}
	if persist {
		if err := s.Store.Insert(ctx, w); err != nil {
			return err
		}
	}
	b, err := json.Marshal(w)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.latest = w
	for ch := range s.clients {
		select {
		case ch <- b:
		default:
			select {
			case <-ch:
			default:
				{
				}
			}
			select {
			case ch <- b:
			default:
				{
				}
			}
		}
	}
	s.mu.Unlock()
	return nil
}
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if s.Token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(s.Token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	latest := s.latest
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"service": "servterm-agent", "version": 1, "node_id": s.NodeID, "latest_at": latest.Sample.At})
}
func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	minutes, _ := strconv.Atoi(r.URL.Query().Get("minutes"))
	if minutes <= 0 || minutes > 43200 {
		minutes = 60
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.Store.History(r.Context(), time.Now().Add(-time.Duration(minutes)*time.Minute), limit)
	if err != nil {
		http.Error(w, "history unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		return
	}
	defer c.CloseNow()
	ch := make(chan []byte, 1)
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	latest := s.latest
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.clients, ch); s.mu.Unlock() }()
	if !latest.Sample.At.IsZero() {
		b, _ := json.Marshal(latest)
		if err := c.Write(r.Context(), websocket.MessageText, b); err != nil {
			return
		}
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case b := <-ch:
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			err := c.Write(ctx, websocket.MessageText, b)
			cancel()
			if err != nil {
				return
			}
		}
	}
}
func ListenAddressSafe(addr, token string) error {
	host := addr
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		host = addr[:i]
	}
	host = strings.Trim(host, "[]")
	if host != "127.0.0.1" && host != "localhost" && host != "::1" && token == "" {
		return fmt.Errorf("SERVTERM_AGENT_TOKEN is required when listening beyond loopback")
	}
	return nil
}
