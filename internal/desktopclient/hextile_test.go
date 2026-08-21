package desktopclient

import (
	"bytes"
	"encoding/binary"
	"testing"

	vnc "github.com/mitchellh/go-vnc"
)

func TestHextileColoredSubrectReadsColorBeforeGeometry(t *testing.T) {
	// Background white, then one red 2x2 colored subrect at (1,1).
	var wire bytes.Buffer
	wire.WriteByte(0x1e) // background + foreground + subrects + colored
	writePixel := func(raw uint32) { _ = binary.Write(&wire, binary.LittleEndian, raw) }
	writePixel(0x00ffffff)
	writePixel(0x00000000)
	wire.WriteByte(1)
	writePixel(0x00ff0000)
	wire.WriteByte(0x11)
	wire.WriteByte(0x11)
	client := &vnc.ClientConn{PixelFormat: vnc.PixelFormat{BPP: 32, Depth: 24, TrueColor: true, RedMax: 255, GreenMax: 255, BlueMax: 255, RedShift: 16, GreenShift: 8, BlueShift: 0}}
	enc, err := (&HextileEncoding{}).Read(client, &vnc.Rectangle{Width: 16, Height: 16}, &wire)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := enc.(*vnc.RawEncoding)
	if !ok {
		t.Fatalf("encoding type %T, want raw", enc)
	}
	if got := raw.Colors[1*16+1]; got.R != 255 || got.G != 0 || got.B != 0 {
		t.Fatalf("colored subrect pixel = %+v, want red", got)
	}
	if got := raw.Colors[0]; got.R != 255 || got.G != 255 || got.B != 255 {
		t.Fatalf("background pixel = %+v, want white", got)
	}
}
