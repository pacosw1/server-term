package ui

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

type desktopImageProto int

const (
	desktopHalfBlock desktopImageProto = iota
	desktopITerm2
	desktopKitty
)

func detectDesktopImageProto() desktopImageProto {
	switch strings.ToLower(os.Getenv("SERVTERM_DESKTOP_RENDER")) {
	case "blocks", "half-block":
		return desktopHalfBlock
	case "iterm2", "iterm":
		return desktopITerm2
	case "kitty":
		return desktopKitty
	}
	if os.Getenv("TERM") == "xterm-kitty" || os.Getenv("KITTY_WINDOW_ID") != "" || os.Getenv("TERM_PROGRAM") == "ghostty" || os.Getenv("TERM") == "xterm-ghostty" {
		return desktopKitty
	}
	if os.Getenv("TERM_PROGRAM") == "iTerm.app" || os.Getenv("TERM_PROGRAM") == "WezTerm" || os.Getenv("TERM_PROGRAM") == "vscode" || os.Getenv("LC_TERMINAL") == "iTerm2" {
		return desktopITerm2
	}
	return desktopHalfBlock
}

func emitDesktopImage(src []byte, cols, rows int) string {
	if len(src) == 0 {
		return ""
	}
	b64 := base64.StdEncoding.EncodeToString(src)
	switch detectDesktopImageProto() {
	case desktopKitty:
		const chunk = 4096
		var out strings.Builder
		out.WriteString("\x1b[s")
		for i := 0; i < len(b64); i += chunk {
			end := i + chunk
			if end > len(b64) {
				end = len(b64)
			}
			more := 1
			if end == len(b64) {
				more = 0
			}
			if i == 0 {
				fmt.Fprintf(&out, "\x1b_Ga=T,i=1,f=100,c=%d,r=%d,C=1,q=2,m=%d;%s\x1b\\", cols, rows, more, b64[i:end])
			} else {
				fmt.Fprintf(&out, "\x1b_Gm=%d;%s\x1b\\", more, b64[i:end])
			}
		}
		out.WriteString("\x1b[u")
		return out.String()
	case desktopITerm2:
		return fmt.Sprintf("\x1b[s\x1b]1337;File=inline=1;size=%d;width=%d;height=%d;preserveAspectRatio=1:%s\x1b\\\x1b[u", len(src), cols, rows, b64)
	default:
		return ""
	}
}
