package desktopclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/franciscosainzwilliams/server-term/internal/config"
)

func FetchScreenshot(ctx context.Context, desktop config.Desktop, token string) ([]byte, error) {
	baseURL, tunnel, err := endpoint(ctx, desktop)
	if err != nil {
		return nil, err
	}
	defer tunnel.Close()
	base := strings.TrimRight(baseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/screenshot", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("desktop screenshot: %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}

type Status struct {
	SchemaVersion int      `json:"schema_version"`
	NodeID        string   `json:"node_id"`
	Platform      string   `json:"platform"`
	Backend       string   `json:"backend"`
	Running       bool     `json:"running"`
	ViewOnly      bool     `json:"view_only"`
	Capabilities  []string `json:"capabilities"`
	Error         string   `json:"error,omitempty"`
}

// Status fetches the desktop agent's authenticated status endpoint. The
// endpoint is deliberately metadata-only; screen/input operations are added
// behind separate capability and confirmation gates.
func FetchStatus(ctx context.Context, desktop config.Desktop, token string) Status {
	out := Status{SchemaVersion: 1, Platform: desktop.Platform, Backend: desktop.Backend}
	baseURL, tunnel, err := endpoint(ctx, desktop)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	defer tunnel.Close()
	base := strings.TrimRight(baseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/status", nil)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		out.Error = fmt.Sprintf("desktop agent: %s", resp.Status)
		return out
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		out.Error = err.Error()
		return out
	}
	return out
}

func (s Status) Healthy() bool { return s.Error == "" && s.Running }
