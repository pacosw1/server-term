package desktopclient

import (
	"context"
	"strings"

	"github.com/coder/websocket"
	"github.com/franciscosainzwilliams/server-term/internal/config"
)

type Stream struct {
	conn   *websocket.Conn
	tunnel *localTunnel
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
	_, b, err := s.conn.Read(ctx)
	return b, err
}
func (s *Stream) Close() {
	if s != nil {
		if s.conn != nil {
			_ = s.conn.Close(websocket.StatusNormalClosure, "client closing")
		}
		s.tunnel.Close()
	}
}
