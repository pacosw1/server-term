package desktopclient

import (
	"encoding/binary"
	"fmt"
	"io"

	vnc "github.com/mitchellh/go-vnc"
)

// HextileEncoding decodes RFC 6143 Hextile tiles into the existing raw-color
// representation used by the framebuffer session.
type HextileEncoding struct{}

func (*HextileEncoding) Type() int32 { return 5 }
func (h *HextileEncoding) Read(c *vnc.ClientConn, rect *vnc.Rectangle, r io.Reader) (vnc.Encoding, error) {
	colors := make([]vnc.Color, int(rect.Width)*int(rect.Height))
	var bg, fg vnc.Color
	for ty := uint16(0); ty < rect.Height; ty += 16 {
		for tx := uint16(0); tx < rect.Width; tx += 16 {
			tw := min16(16, rect.Width-tx)
			th := min16(16, rect.Height-ty)
			var sub [1]byte
			if _, err := io.ReadFull(r, sub[:]); err != nil {
				return nil, err
			}
			bits := sub[0]
			if bits&1 != 0 {
				for y := uint16(0); y < th; y++ {
					for x := uint16(0); x < tw; x++ {
						col, err := readPixel(c, r)
						if err != nil {
							return nil, err
						}
						colors[int(ty+y)*int(rect.Width)+int(tx+x)] = col
					}
				}
				continue
			}
			if bits&2 != 0 {
				var err error
				bg, err = readPixel(c, r)
				if err != nil {
					return nil, err
				}
			}
			for y := uint16(0); y < th; y++ {
				for x := uint16(0); x < tw; x++ {
					colors[int(ty+y)*int(rect.Width)+int(tx+x)] = bg
				}
			}
			if bits&4 != 0 {
				var err error
				fg, err = readPixel(c, r)
				if err != nil {
					return nil, err
				}
			}
			if bits&8 != 0 {
				var n [1]byte
				if _, err := io.ReadFull(r, n[:]); err != nil {
					return nil, err
				}
				for i := 0; i < int(n[0]); i++ {
					col := fg
					if bits&16 != 0 {
						var err error
						col, err = readPixel(c, r)
						if err != nil {
							return nil, err
						}
					}
					var xywh [2]byte
					if _, err := io.ReadFull(r, xywh[:]); err != nil {
						return nil, err
					}
					sx := uint16(xywh[0] >> 4)
					sy := uint16(xywh[0] & 15)
					sw := uint16((xywh[1] >> 4) + 1)
					sh := uint16((xywh[1] & 15) + 1)
					for y := uint16(0); y < sh && sy+y < th; y++ {
						for x := uint16(0); x < sw && sx+x < tw; x++ {
							colors[int(ty+sy+y)*int(rect.Width)+int(tx+sx+x)] = col
						}
					}
				}
			}
		}
	}
	return &vnc.RawEncoding{Colors: colors}, nil
}
func min16(a, b uint16) uint16 {
	if a < b {
		return a
	}
	return b
}
func readPixel(c *vnc.ClientConn, r io.Reader) (vnc.Color, error) {
	n := int(c.PixelFormat.BPP / 8)
	var storage [4]byte
	if n < 1 || n > len(storage) {
		return vnc.Color{}, fmt.Errorf("unsupported VNC pixel width %d", n)
	}
	buf := storage[:n]
	if _, err := io.ReadFull(r, buf); err != nil {
		return vnc.Color{}, err
	}
	var raw uint32
	switch c.PixelFormat.BPP {
	case 8:
		raw = uint32(buf[0])
	case 16:
		var order binary.ByteOrder = binary.LittleEndian
		if c.PixelFormat.BigEndian {
			order = binary.BigEndian
		}
		raw = uint32(order.Uint16(buf))
	case 32:
		var order binary.ByteOrder = binary.LittleEndian
		if c.PixelFormat.BigEndian {
			order = binary.BigEndian
		}
		raw = order.Uint32(buf)
	}
	if !c.PixelFormat.TrueColor {
		return c.ColorMap[raw], nil
	}
	return vnc.Color{R: uint16((raw >> c.PixelFormat.RedShift) & uint32(c.PixelFormat.RedMax)), G: uint16((raw >> c.PixelFormat.GreenShift) & uint32(c.PixelFormat.GreenMax)), B: uint16((raw >> c.PixelFormat.BlueShift) & uint32(c.PixelFormat.BlueMax))}, nil
}
