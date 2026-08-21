package desktopclient

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/franciscosainzwilliams/server-term/internal/config"
	vnc "github.com/mitchellh/go-vnc"
)

// Screenshot connects directly to the configured RFB endpoint and captures one
// framebuffer. It is intentionally one-shot and view-only; input control is a
// separate, explicit capability that is not enabled by this path.
func Screenshot(ctx context.Context, desktop config.Desktop, password, output string) error {
	port := desktop.VNCPort
	if port == 0 {
		port = 5900
	}
	pngData, err := Capture(ctx, desktop.Host, port, password)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0700); err != nil && filepath.Dir(output) != "." {
		return err
	}
	return os.WriteFile(output, pngData, 0600)
}

func Capture(ctx context.Context, host string, port int, password string) ([]byte, error) {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	messages := make(chan vnc.ServerMessage, 8)
	none := vnc.ClientAuthNone(0)
	auth := []vnc.ClientAuth{&none}
	if password != "" {
		auth = []vnc.ClientAuth{&vnc.PasswordAuth{Password: password}, &none}
	}
	client, err := vnc.Client(conn, &vnc.ClientConfig{Auth: auth, ServerMessageCh: messages, ServerMessages: []vnc.ServerMessage{&vnc.FramebufferUpdateMessage{}, &vnc.ServerCutTextMessage{}}})
	if err != nil {
		return nil, err
	}
	defer client.Close()
	if err := client.SetPixelFormat(&vnc.PixelFormat{BPP: 32, Depth: 24, BigEndian: false, TrueColor: true, RedMax: 255, GreenMax: 255, BlueMax: 255, RedShift: 16, GreenShift: 8, BlueShift: 0}); err != nil {
		return nil, err
	}
	_ = client.SetEncodings([]vnc.Encoding{&HextileEncoding{}, &vnc.RawEncoding{}})
	if err := client.FramebufferUpdateRequest(false, 0, 0, client.FrameBufferWidth, client.FrameBufferHeight); err != nil {
		return nil, err
	}
	var update *vnc.FramebufferUpdateMessage
	select {
	case msg := <-messages:
		var ok bool
		update, ok = msg.(*vnc.FramebufferUpdateMessage)
		if !ok {
			return nil, fmt.Errorf("unexpected VNC message %T", msg)
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	img := image.NewRGBA(image.Rect(0, 0, int(client.FrameBufferWidth), int(client.FrameBufferHeight)))
	for _, rect := range update.Rectangles {
		raw, ok := rect.Enc.(*vnc.RawEncoding)
		if !ok {
			continue
		}
		for i, color := range raw.Colors {
			x := int(rect.X) + i%int(rect.Width)
			y := int(rect.Y) + i/int(rect.Width)
			if x < img.Rect.Max.X && y < img.Rect.Max.Y {
				img.SetRGBA(x, y, colorToRGBA(color))
			}
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// CaptureSession keeps one RFB connection and framebuffer alive. Call Next for
// incremental updates instead of reconnecting for every frame.
type CaptureSession struct {
	conn     net.Conn
	client   *vnc.ClientConn
	messages chan vnc.ServerMessage
	img      *image.RGBA
	first    bool
	mu       sync.Mutex
}

func NewCaptureSession(ctx context.Context, host string, port int, password string) (*CaptureSession, error) {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	messages := make(chan vnc.ServerMessage, 16)
	none := vnc.ClientAuthNone(0)
	auth := []vnc.ClientAuth{&none}
	if password != "" {
		auth = []vnc.ClientAuth{&vnc.PasswordAuth{Password: password}, &none}
	}
	client, err := vnc.Client(conn, &vnc.ClientConfig{Auth: auth, ServerMessageCh: messages, ServerMessages: []vnc.ServerMessage{&vnc.FramebufferUpdateMessage{}, &vnc.ServerCutTextMessage{}}})
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := client.SetPixelFormat(&vnc.PixelFormat{BPP: 32, Depth: 24, BigEndian: false, TrueColor: true, RedMax: 255, GreenMax: 255, BlueMax: 255, RedShift: 16, GreenShift: 8, BlueShift: 0}); err != nil {
		_ = client.Close()
		return nil, err
	}
	_ = client.SetEncodings([]vnc.Encoding{&HextileEncoding{}, &vnc.RawEncoding{}})
	return &CaptureSession{conn: conn, client: client, messages: messages, img: image.NewRGBA(image.Rect(0, 0, int(client.FrameBufferWidth), int(client.FrameBufferHeight))), first: true}, nil
}
func (s *CaptureSession) Close() {
	if s != nil {
		if s.client != nil {
			_ = s.client.Close()
		}
		if s.conn != nil {
			_ = s.conn.Close()
		}
	}
}
func (s *CaptureSession) Next(ctx context.Context) ([]byte, error) {
	s.mu.Lock()
	if s.first {
		if err := s.client.FramebufferUpdateRequest(false, 0, 0, s.client.FrameBufferWidth, s.client.FrameBufferHeight); err != nil {
			s.mu.Unlock()
			return nil, err
		}
		s.first = false
	} else {
		if err := s.client.FramebufferUpdateRequest(true, 0, 0, s.client.FrameBufferWidth, s.client.FrameBufferHeight); err != nil {
			s.mu.Unlock()
			return nil, err
		}
	}
	s.mu.Unlock()
	for {
		select {
		case msg := <-s.messages:
			if cut, ok := msg.(*vnc.ServerCutTextMessage); ok {
				_ = cut
				continue
			}
			update, ok := msg.(*vnc.FramebufferUpdateMessage)
			if !ok {
				return nil, fmt.Errorf("unexpected VNC message %T", msg)
			}
			s.mu.Lock()
			for _, rect := range update.Rectangles {
				raw, ok := rect.Enc.(*vnc.RawEncoding)
				if !ok {
					continue
				}
				for i, c := range raw.Colors {
					x := int(rect.X) + i%int(rect.Width)
					y := int(rect.Y) + i/int(rect.Width)
					if x < s.img.Rect.Max.X && y < s.img.Rect.Max.Y {
						s.img.SetRGBA(x, y, colorToRGBA(c))
					}
				}
			}
			var b bytes.Buffer
			if err := png.Encode(&b, s.img); err != nil {
				s.mu.Unlock()
				return nil, err
			}
			s.mu.Unlock()
			return b.Bytes(), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
func (s *CaptureSession) Key(keysym uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.client.KeyEvent(keysym, true); err != nil {
		return err
	}
	return s.client.KeyEvent(keysym, false)
}
func (s *CaptureSession) Pointer(x, y uint16, mask vnc.ButtonMask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client.PointerEvent(mask, x, y)
}
func (s *CaptureSession) CutText(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client.CutText(text)
}

func sendVNC(ctx context.Context, host string, port int, password string, fn func(*vnc.ClientConn) error) error {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	none := vnc.ClientAuthNone(0)
	auth := []vnc.ClientAuth{&none}
	if password != "" {
		auth = []vnc.ClientAuth{&vnc.PasswordAuth{Password: password}, &none}
	}
	client, err := vnc.Client(conn, &vnc.ClientConfig{Auth: auth})
	if err != nil {
		return err
	}
	defer client.Close()
	return fn(client)
}

func SendPointer(ctx context.Context, host string, port int, password string, x, y uint16, mask vnc.ButtonMask) error {
	return sendVNC(ctx, host, port, password, func(c *vnc.ClientConn) error { return c.PointerEvent(mask, x, y) })
}
func SendVNCKey(ctx context.Context, host string, port int, password string, keysym uint32) error {
	return sendVNC(ctx, host, port, password, func(c *vnc.ClientConn) error {
		if err := c.KeyEvent(keysym, true); err != nil {
			return err
		}
		return c.KeyEvent(keysym, false)
	})
}

func colorToRGBA(c vnc.Color) color.RGBA {
	// go-vnc exposes channel values in the negotiated channel range (0..255
	// for our 8-bit channels), not normalized 16-bit color components.
	return color.RGBA{R: uint8(c.R), G: uint8(c.G), B: uint8(c.B), A: 255}
}
