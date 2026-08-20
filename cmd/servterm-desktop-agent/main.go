// servterm-desktop-agent is the least-privilege desktop control-plane agent.
// It reports platform/backend capabilities and refuses to advertise control
// until a supported native capture backend is installed and configured.
package main

import (
	"crypto/subtle"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
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
	s := probe(*node, *platform, *backend)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/status", auth(token, func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, s) }))
	mux.HandleFunc("GET /v1/capabilities", auth(token, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"schema_version": 1, "capabilities": s.Capabilities})
	}))
	server := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 3 * time.Second}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fatal(err)
	}
}

func probe(node, platform, requested string) status {
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
	running := b != "unsupported" && b != "none"
	caps := []string{"status", "capabilities"}
	if running {
		caps = append(caps, "vnc")
	}
	return status{SchemaVersion: 1, NodeID: node, Platform: platform, Backend: b, Running: running, ViewOnly: true, Capabilities: caps, At: time.Now(), Error: map[bool]string{true: "", false: "no supported native desktop backend detected"}[running]}
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
