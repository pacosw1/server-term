package pair

import (
	"strings"

	"rsc.io/qr"
)

// quiet is the white margin, in modules, that a scanner needs round a code.
const quiet = 2

// RenderQR draws text as a QR code with half blocks, two rows of modules
// to one line of text. It sets both colours itself, so the code reads the
// same in a dark and in a light terminal.
func RenderQR(text string) (string, error) {
	code, err := qr.Encode(text, qr.L)
	if err != nil {
		return "", err
	}
	dark := func(x, y int) bool {
		if x < 0 || y < 0 || x >= code.Size || y >= code.Size {
			return false
		}
		return code.Black(x, y)
	}
	var b strings.Builder
	for y := -quiet; y < code.Size+quiet; y += 2 {
		// White foreground draws the light modules; black background
		// shows through for the dark ones.
		b.WriteString("\x1b[97;40m")
		for x := -quiet; x < code.Size+quiet; x++ {
			top, bottom := !dark(x, y), !dark(x, y+1)
			switch {
			case top && bottom:
				b.WriteString("█")
			case top:
				b.WriteString("▀")
			case bottom:
				b.WriteString("▄")
			default:
				b.WriteString(" ")
			}
		}
		b.WriteString("\x1b[0m\n")
	}
	return b.String(), nil
}
