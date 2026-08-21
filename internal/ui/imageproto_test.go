package ui

import (
	"os"
	"strings"
	"testing"
)

func TestEmitDesktopImageProtocols(t *testing.T) {
	old := os.Getenv("SERVTERM_DESKTOP_RENDER")
	defer os.Setenv("SERVTERM_DESKTOP_RENDER", old)
	for _, tc := range []struct{ name, value, prefix string }{{"kitty", "kitty", "\x1b_G"}, {"iterm", "iterm2", "\x1b]1337;"}} {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.Setenv("SERVTERM_DESKTOP_RENDER", tc.value)
			got := emitDesktopImage([]byte("png"), 10, 5)
			if !strings.Contains(got, tc.prefix) {
				t.Fatalf("output missing %q: %q", tc.prefix, got)
			}
		})
	}
}

func TestClearDesktopImageOnlyTargetsKittyPlacement(t *testing.T) {
	old := os.Getenv("SERVTERM_DESKTOP_RENDER")
	defer os.Setenv("SERVTERM_DESKTOP_RENDER", old)
	_ = os.Setenv("SERVTERM_DESKTOP_RENDER", "kitty")
	if got := clearDesktopImage(); got != "\x1b_Ga=d,d=I,i=1,q=2\x1b\\" {
		t.Fatalf("kitty clear sequence = %q", got)
	}
	_ = os.Setenv("SERVTERM_DESKTOP_RENDER", "iterm2")
	if got := clearDesktopImage(); got != "" {
		t.Fatalf("non-Kitty clear sequence = %q", got)
	}
}
