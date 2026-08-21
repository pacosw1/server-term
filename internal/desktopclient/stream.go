package desktopclient

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/coder/websocket"
	"github.com/franciscosainzwilliams/server-term/internal/config"
)

type Stream struct {
	conn      *websocket.Conn
	tunnel    *localTunnel
	clipboard string
}

func OpenStream(ctx context.Context, desktop config.Desktop, token string) (*Stream, error) {
	base, tunnel, err := endpoint(ctx, desktop)
	if err != nil {
		return nil, err
	}
	url := strings.Replace(base, "http://", "ws://", 1) + "/v1/stream"
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: map[string][]string{"Authorization": {"Bearer " + token}}})
	if err != nil {
		tunnel.Close()
		return nil, err
	}
	return &Stream{conn: conn, tunnel: tunnel}, nil
}
func (s *Stream) Read(ctx context.Context) ([]byte, error) {
	for {
		kind, b, err := s.conn.Read(ctx)
		if err != nil {
			return nil, err
		}
		if kind == websocket.MessageText {
			var msg struct{ Type, Text string }
			if json.Unmarshal(b, &msg) == nil && msg.Type == "clipboard" {
				s.clipboard = msg.Text
			}
			continue
		}
		return b, nil
	}
}
func (s *Stream) Clipboard() string { text := s.clipboard; s.clipboard = ""; return text }
func (s *Stream) send(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.conn.Write(context.Background(), websocket.MessageText, b)
}
func (s *Stream) Key(combo string) error {
	return s.send(map[string]any{"type": "key", "combo": combo})
}
func (s *Stream) Click(x, y int, right bool) error {
	button := 1
	if right {
		button = 3
	}
	return s.send(map[string]any{"type": "click", "x": x, "y": y, "button": button})
}
func (s *Stream) ClipboardSet(text string) error {
	return s.send(map[string]any{"type": "clipboard_set", "text": text})
}
func (s *Stream) Close() {
	if s != nil {
		if s.conn != nil {
			_ = s.conn.Close(websocket.StatusNormalClosure, "client closing")
		}
		s.tunnel.Close()
	}
}
