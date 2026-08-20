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
	client, err := vnc.Client(conn, &vnc.ClientConfig{Auth: auth, ServerMessageCh: messages, ServerMessages: []vnc.ServerMessage{&vnc.FramebufferUpdateMessage{}}})
	if err != nil {
		return nil, err
	}
	defer client.Close()
	_ = client.SetEncodings([]vnc.Encoding{&vnc.RawEncoding{}})
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

func colorToRGBA(c vnc.Color) color.RGBA {
	return color.RGBA{R: uint8(c.R >> 8), G: uint8(c.G >> 8), B: uint8(c.B >> 8), A: 255}
}
