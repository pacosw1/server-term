package agentclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/franciscosainzwilliams/server-term/internal/metrics"
)

type Stream struct{ Conn *websocket.Conn }

func History(ctx context.Context, baseURL, token string, span time.Duration, limit int) ([]metrics.Sample, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/v1/history"
	q := u.Query()
	q.Set("minutes", fmt.Sprint(max(1, int(span.Minutes()))))
	q.Set("limit", fmt.Sprint(limit))
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent history: %s", resp.Status)
	}
	var wire []metrics.WireSample
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return nil, err
	}
	out := make([]metrics.Sample, 0, len(wire))
	for _, w := range wire {
		if w.Version == 1 {
			out = append(out, w.Sample)
		}
	}
	return out, nil
}
func Connect(ctx context.Context, baseURL, token string) (*Stream, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return nil, fmt.Errorf("agent URL must use http, https, ws, or wss")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/v1/stream"
	h := http.Header{}
	h.Set("Authorization", "Bearer "+token)
	conn, resp, err := websocket.Dial(ctx, u.String(), &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("agent stream: %s", resp.Status)
		}
		return nil, err
	}
	return &Stream{Conn: conn}, nil
}
func (s *Stream) Read(ctx context.Context) (metrics.WireSample, error) {
	_, b, err := s.Conn.Read(ctx)
	if err != nil {
		return metrics.WireSample{}, err
	}
	var w metrics.WireSample
	if err := json.Unmarshal(b, &w); err != nil {
		return w, err
	}
	if w.Version != 1 {
		return w, fmt.Errorf("unsupported agent protocol %d", w.Version)
	}
	w.Sample.Latency = time.Since(w.Sample.At)
	if w.Sample.Latency < 0 {
		w.Sample.Latency = 0
	}
	return w, nil
}
func (s *Stream) Close() { _ = s.Conn.Close(websocket.StatusNormalClosure, "client closing") }
