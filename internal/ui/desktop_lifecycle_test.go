package ui

import (
	stdbytes "bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/franciscosainzwilliams/server-term/internal/config"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var b stdbytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestDesktopFrameRerendersOnTerminalResize(t *testing.T) {
	old := os.Getenv("SERVTERM_DESKTOP_RENDER")
	defer os.Setenv("SERVTERM_DESKTOP_RENDER", old)
	_ = os.Setenv("SERVTERM_DESKTOP_RENDER", "kitty")
	m := New(config.Config{
		Servers:  []config.Server{{Name: "ci", Address: "ci"}},
		Desktops: []config.Desktop{{Name: "ci-desktop", Host: "ci"}},
	})
	m.samples[0].Online, m.samples[0].At = true, time.Now()
	m.detail, m.detailTab, m.width, m.height = true, 7, 80, 30
	m.desktopFrameRaw[0] = testPNG(t)
	m.desktopFrames[0] = m.renderDesktopFrame(0, m.desktopFrameRaw[0])
	if !strings.Contains(m.desktopFrames[0], "c=76,r=18") {
		t.Fatalf("initial placement missing: %q", m.desktopFrames[0][:min(120, len(m.desktopFrames[0]))])
	}
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	resized := model.(Model)
	if !strings.Contains(resized.desktopFrames[0], "c=110,r=28") {
		t.Fatalf("resized placement missing: %q", resized.desktopFrames[0][:min(120, len(resized.desktopFrames[0]))])
	}
}

func TestDesktopLeavingTabEmitsKittyCleanup(t *testing.T) {
	old := os.Getenv("SERVTERM_DESKTOP_RENDER")
	defer os.Setenv("SERVTERM_DESKTOP_RENDER", old)
	_ = os.Setenv("SERVTERM_DESKTOP_RENDER", "kitty")
	m := New(config.Config{
		Servers:  []config.Server{{Name: "ci", Address: "ci"}},
		Desktops: []config.Desktop{{Name: "ci-desktop", Host: "ci"}},
	})
	m.samples[0].Online, m.samples[0].At = true, time.Now()
	m.detail, m.detailTab, m.width, m.height = true, 7, 80, 30
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	left := model.(Model)
	if left.detailTab == 7 || left.desktopClear != "\x1b_Ga=d,d=I,i=1,q=2\x1b\\" {
		t.Fatalf("desktop cleanup not queued: tab=%d clear=%q", left.detailTab, left.desktopClear)
	}
	if !strings.Contains(left.detailView(), "\x1b_Ga=d,d=I,i=1,q=2") {
		t.Fatal("detail view did not carry cleanup sequence")
	}
}

func TestDesktopEnterIsHandledByTUI(t *testing.T) {
	if isRemoteDesktopKey("enter") {
		t.Fatal("enter must not be forwarded as remote desktop input")
	}
	m := New(config.Config{
		Servers:  []config.Server{{Name: "ci", Address: "ci"}},
		Desktops: []config.Desktop{{Name: "ci-desktop", Host: "ci"}},
	})
	m.samples[0].Online, m.samples[0].At = true, time.Now()
	m.detail, m.detailTab = true, 7
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on the desktop tab did not start a command")
	}
}

func TestDesktopMouseCoordinatesFollowRenderedImage(t *testing.T) {
	old := os.Getenv("SERVTERM_DESKTOP_RENDER")
	defer os.Setenv("SERVTERM_DESKTOP_RENDER", old)
	_ = os.Setenv("SERVTERM_DESKTOP_RENDER", "kitty")
	m := New(config.Config{
		Servers:  []config.Server{{Name: "ci", Address: "ci"}},
		Desktops: []config.Desktop{{Name: "ci-desktop", Host: "ci"}},
	})
	m.samples[0].Online, m.samples[0].At = true, time.Now()
	m.detail, m.detailTab, m.width, m.height = true, 7, 80, 30
	m.desktopFrameRaw[0] = testPNG(t)
	m.desktopFrameSize[0] = image.Point{X: 1280, Y: 800}
	m.desktopFrames[0] = m.renderDesktopFrame(0, m.desktopFrameRaw[0])
	ox, oy, ok := m.desktopImageOrigin(0)
	if !ok {
		t.Fatal("desktop image origin unavailable")
	}
	cols, rows := m.desktopImageCells(0)
	rx, ry, ok := m.desktopRemotePoint(0, ox+cols/2, oy+rows/2)
	if !ok || rx < 630 || rx > 650 || ry < 390 || ry > 410 {
		t.Fatalf("center click mapped to (%d,%d), ok=%v", rx, ry, ok)
	}
}
