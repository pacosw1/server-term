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
	"github.com/franciscosainzwilliams/server-term/internal/desktopclient"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	server := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 3 * time.Second}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fatal(err)
	}
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
		caps = append(caps, "vnc")
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
