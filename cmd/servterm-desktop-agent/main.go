// servterm-desktop-agent is the least-privilege desktop control-plane agent.
// It reports platform/backend capabilities and refuses to advertise control
// until a supported native capture backend is installed and configured.
package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/coder/websocket"
	"github.com/franciscosainzwilliams/server-term/internal/desktopclient"
	vnc "github.com/mitchellh/go-vnc"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type status struct {
	SchemaVersion int       `json:"schema_version"`
	NodeID        string    `json:"node_id"`
	Platform      string    `json:"platform"`
	Backend       string    `json:"backend"`
	Running       bool      `json:"running"`
	ViewOnly      bool      `json:"view_only"`
	Capabilities  []string  `json:"capabilities"`
	At            time.Time `json:"at"`
	Error         string    `json:"error,omitempty"`
}

func main() {
	listen := flag.String("listen", "127.0.0.1:7850", "HTTP listen address")
	node := flag.String("node", hostname(), "stable node name")
	platform := flag.String("platform", runtime.GOOS, "platform label: macos, linux, or windows")
	backend := flag.String("backend", "auto", "native desktop backend")
	tokenFile := flag.String("token-file", "", "bearer token file")
	vncHost := flag.String("vnc-host", "127.0.0.1", "native VNC backend host")
	vncPort := flag.Int("vnc-port", 5900, "native VNC backend port")
	vncPasswordFile := flag.String("vnc-password-file", "", "native VNC password file")
	captureBackend := flag.String("capture-backend", "auto", "capture backend: auto, native, or vnc")
	allowInput := flag.Bool("allow-input", false, "enable explicit keyboard/mouse control")
	flag.Parse()
	token := ""
	if *tokenFile != "" {
		b, err := os.ReadFile(*tokenFile)
		if err != nil {
			fatal(err)
		}
		token = strings.TrimSpace(string(b))
	}
	if !isLoopback(*listen) && token == "" {
		fatal(fmt.Errorf("non-loopback listen requires --token-file"))
	}
	s := probe(*node, *platform, *backend, *vncHost, *vncPort)
	if *allowInput {
		s.ViewOnly = false
		s.Capabilities = append(s.Capabilities, "input")
	}
	if *captureBackend == "native" || *captureBackend == "auto" {
		if nativeCaptureAvailable(*platform) {
			s.Running = true
			s.Error = ""
			s.Capabilities = append(s.Capabilities, "screenshot")
		}
	}
	vncPassword := ""
	if *vncPasswordFile != "" {
		b, err := os.ReadFile(*vncPasswordFile)
		if err != nil && !os.IsNotExist(err) {
			fatal(err)
		}
		if err == nil {
			vncPassword = strings.TrimSpace(string(b))
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/status", auth(token, func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, s) }))
	mux.HandleFunc("GET /v1/capabilities", auth(token, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"schema_version": 1, "capabilities": s.Capabilities})
	}))
	mux.HandleFunc("GET /v1/screenshot", auth(token, func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		if *captureBackend == "native" || (*captureBackend == "auto" && (*platform == "macos" || *platform == "darwin")) {
			data, err := nativeScreenshot(ctx, *platform)
			if err == nil {
				w.Header().Set("Content-Type", "image/png")
				w.Header().Set("Cache-Control", "no-store")
				_, _ = w.Write(data)
				return
			}
			if *captureBackend == "native" {
				http.Error(w, "native screenshot unavailable: "+err.Error(), http.StatusServiceUnavailable)
				return
			}
		}
		data, err := desktopclient.Capture(ctx, *vncHost, *vncPort, vncPassword)
		if err != nil {
			http.Error(w, "screenshot unavailable: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
	}))
	mux.HandleFunc("GET /v1/stream", auth(token, func(w http.ResponseWriter, r *http.Request) { serveStream(w, r, *vncHost, *vncPort, vncPassword) }))
	mux.HandleFunc("POST /v1/key", auth(token, func(w http.ResponseWriter, r *http.Request) {
		if !*allowInput || r.Header.Get("X-Servterm-Confirm") != "yes" {
			http.Error(w, "input disabled or confirmation missing", http.StatusForbidden)
			return
		}
		key, ok := keySym(r.URL.Query().Get("combo"))
		if !ok {
			http.Error(w, "unsupported key combo", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := desktopclient.SendVNCKey(ctx, *vncHost, *vncPort, vncPassword, key); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}))
	mux.HandleFunc("POST /v1/click", auth(token, func(w http.ResponseWriter, r *http.Request) {
		if !*allowInput || r.Header.Get("X-Servterm-Confirm") != "yes" {
			http.Error(w, "input disabled or confirmation missing", http.StatusForbidden)
			return
		}
		x, xerr := strconv.Atoi(r.URL.Query().Get("x"))
		y, yerr := strconv.Atoi(r.URL.Query().Get("y"))
		if xerr != nil || yerr != nil || x < 0 || y < 0 || x > 65535 || y > 65535 {
			http.Error(w, "invalid coordinates", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := desktopclient.SendPointer(ctx, *vncHost, *vncPort, vncPassword, uint16(x), uint16(y), 1); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}))
	server := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 3 * time.Second}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fatal(err)
	}
}

func serveStream(w http.ResponseWriter, r *http.Request, host string, port int, password string) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "stream closed")
	streamCtx, cancel := context.WithCancel(r.Context())
	defer cancel()
	session, err := desktopclient.NewCaptureSession(streamCtx, host, port, password)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, err.Error())
		return
	}
	defer session.Close()
	go func() {
		for {
			_, data, e := conn.Read(streamCtx)
			if e != nil {
				cancel()
				return
			}
			var control struct {
				Type   string `json:"type"`
				Combo  string `json:"combo"`
				Text   string `json:"text"`
				X      int    `json:"x"`
				Y      int    `json:"y"`
				Button int    `json:"button"`
			}
			if json.Unmarshal(data, &control) != nil {
				continue
			}
			switch control.Type {
			case "key":
				if key, ok := keySym(control.Combo); ok {
					_ = session.Key(key)
				}
			case "click":
				if control.X >= 0 && control.Y >= 0 {
					mask := vnc.ButtonMask(1)
					if control.Button == 3 {
						mask = 4
					}
					_ = session.Pointer(uint16(control.X), uint16(control.Y), mask)
					_ = session.Pointer(uint16(control.X), uint16(control.Y), 0)
				}
			case "clipboard_set":
				_ = session.CutText(control.Text)
			}
		}
	}()
	for {
		frame, err := session.Next(streamCtx)
		if err != nil {
			return
		}
		if err := conn.Write(streamCtx, websocket.MessageBinary, frame); err != nil {
			return
		}
		if clipboard := session.TakeClipboard(); clipboard != "" {
			payload, _ := json.Marshal(map[string]any{"type": "clipboard", "text": clipboard})
			if err := conn.Write(streamCtx, websocket.MessageText, payload); err != nil {
				return
			}
		}
	}
}

func keySym(combo string) (uint32, bool) {
	keys := map[string]uint32{"enter": 0xff0d, "return": 0xff0d, "esc": 0xff1b, "escape": 0xff1b, "tab": 0xff09, "backspace": 0xff08, "delete": 0xffff, "left": 0xff51, "up": 0xff52, "right": 0xff53, "down": 0xff54, "f1": 0xffbe, "f2": 0xffbf, "f3": 0xffc0, "f4": 0xffc1, "f5": 0xffc2, "f6": 0xffc3, "f7": 0xffc4, "f8": 0xffc5, "f9": 0xffc6, "f10": 0xffc7, "f11": 0xffc8, "f12": 0xffc9, "space": 0x20}
	if value, ok := keys[strings.ToLower(combo)]; ok {
		return value, true
	}
	runes := []rune(combo)
	if len(runes) == 1 {
		return uint32(runes[0]), true
	}
	return 0, false
}

func nativeScreenshot(ctx context.Context, platform string) ([]byte, error) {
	command := ""
	var args []string
	switch platform {
	case "macos", "darwin":
		command = "screencapture"
	case "linux":
		for _, candidate := range []string{"gnome-screenshot", "scrot", "import"} {
			if _, err := exec.LookPath(candidate); err == nil {
				command = candidate
				break
			}
		}
		if command == "" {
			return nil, fmt.Errorf("no Linux screenshot command installed")
		}
	default:
		return nil, fmt.Errorf("native capture unsupported on %s", platform)
	}
	tmp, err := os.CreateTemp("", "servterm-desktop-*.png")
	if err != nil {
		return nil, err
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)
	switch command {
	case "screencapture":
		args = []string{"-x", "-t", "png", path}
	case "gnome-screenshot":
		args = []string{"-f", path}
	case "scrot":
		args = []string{path}
	case "import":
		args = []string{"-window", "root", path}
	}
	if out, err := exec.CommandContext(ctx, command, args...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%s: %w: %s", command, err, strings.TrimSpace(string(out)))
	}
	return os.ReadFile(filepath.Clean(path))
}
func nativeCaptureAvailable(platform string) bool {
	if platform == "macos" || platform == "darwin" {
		if _, err := exec.LookPath("screencapture"); err != nil {
			return false
		}
		tmp, err := os.CreateTemp("", "servterm-probe-*.png")
		if err != nil {
			return false
		}
		path := tmp.Name()
		_ = tmp.Close()
		defer os.Remove(path)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return exec.CommandContext(ctx, "screencapture", "-x", "-t", "png", path).Run() == nil
	}
	if platform == "linux" {
		for _, name := range []string{"gnome-screenshot", "scrot", "import"} {
			if _, err := exec.LookPath(name); err == nil {
				return true
			}
		}
	}
	return false
}

func probe(node, platform, requested, vncHost string, vncPort int) status {
	b := requested
	if b == "auto" {
		switch platform {
		case "darwin", "macos":
			b = "screensharing"
		case "linux":
			b = detectLinuxBackend()
		default:
			b = "unsupported"
		}
	}
	running := b != "unsupported" && b != "none" && canDial(vncHost, vncPort)
	caps := []string{"status", "capabilities"}
	if running {
		caps = append(caps, "vnc", "screenshot")
	}
	return status{SchemaVersion: 1, NodeID: node, Platform: platform, Backend: b, Running: running, ViewOnly: true, Capabilities: caps, At: time.Now(), Error: map[bool]string{true: "", false: "no supported native desktop backend detected"}[running]}
}
func canDial(host string, port int) bool {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}
func detectLinuxBackend() string {
	for _, name := range []string{"wayvnc", "x11vnc", "gnome-remote-desktop"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return "unsupported"
}
func auth(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token != "" {
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if len(got) != len(token) || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	return err == nil && (host == "127.0.0.1" || host == "::1" || host == "localhost")
}
func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "desktop"
	}
	return h
}
func fatal(err error) { fmt.Fprintln(os.Stderr, "servterm-desktop-agent:", err); os.Exit(1) }
