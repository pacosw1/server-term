package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/franciscosainzwilliams/server-term/internal/agentclient"
	"github.com/franciscosainzwilliams/server-term/internal/config"
	"github.com/franciscosainzwilliams/server-term/internal/desktopclient"
	"github.com/franciscosainzwilliams/server-term/internal/widget"
)

type Server struct{ Config config.Config }
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}
type response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}
type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func (s Server) Run(ctx context.Context, in io.Reader, out io.Writer) error {
	dec := json.NewDecoder(in)
	enc := json.NewEncoder(out)
	for {
		var req request
		if err := dec.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if req.Method == "notifications/initialized" {
			continue
		}
		result, err := s.handle(ctx, req.Method, req.Params)
		if req.ID == nil {
			continue
		}
		resp := response{JSONRPC: "2.0", ID: req.ID}
		if err != nil {
			resp.Error = map[string]any{"code": -32000, "message": err.Error()}
		} else {
			resp.Result = result
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
}

func (s Server) handle(ctx context.Context, method string, raw json.RawMessage) (any, error) {
	switch method {
	case "initialize":
		return map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "servterm", "version": "dev"}}, nil
	case "tools/list":
		return map[string]any{"tools": tools()}, nil
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		return s.call(ctx, p.Name, p.Arguments)
	default:
		return nil, fmt.Errorf("unsupported MCP method %q", method)
	}
}

func tools() []tool {
	schema := func(props map[string]any, required []string) map[string]any {
		if required == nil {
			required = []string{}
		}
		return map[string]any{"type": "object", "properties": props, "required": required}
	}
	return []tool{
		{Name: "servterm_list_servers", Description: "List configured Servterm servers without credentials.", InputSchema: schema(map[string]any{}, nil)},
		{Name: "servterm_status", Description: "Get the latest authenticated host metrics for one or all configured servers.", InputSchema: schema(map[string]any{"server": map[string]any{"type": "string"}}, nil)},
		{Name: "servterm_history", Description: "Read recent authenticated host metric samples.", InputSchema: schema(map[string]any{"server": map[string]any{"type": "string"}, "minutes": map[string]any{"type": "integer", "minimum": 1, "maximum": 1440}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 600}}, []string{"server"})},
		{Name: "servterm_stream", Description: "Read a bounded number of live authenticated samples; never creates an unbounded background stream.", InputSchema: schema(map[string]any{"server": map[string]any{"type": "string"}, "samples": map[string]any{"type": "integer", "minimum": 1, "maximum": 10}}, []string{"server"})},
		{Name: "servterm_list_desktops", Description: "List configured desktop agents and capabilities without credentials.", InputSchema: schema(map[string]any{}, nil)},
		{Name: "servterm_desktop_status", Description: "Read authenticated desktop-agent capability status.", InputSchema: schema(map[string]any{"desktop": map[string]any{"type": "string"}}, []string{"desktop"})},
		{Name: "servterm_nvr_status", Description: "Read a configured read-only NVR widget snapshot.", InputSchema: schema(map[string]any{"widget": map[string]any{"type": "string"}}, []string{"widget"})},
	}
}

func (s Server) call(ctx context.Context, name string, args map[string]any) (any, error) {
	switch name {
	case "servterm_list_servers":
		out := []map[string]any{}
		for _, v := range s.Config.Servers {
			out = append(out, map[string]any{"name": v.Name, "address": v.Address, "user": v.User, "location": v.Location, "tags": v.Tags, "agent_configured": v.AgentURL != ""})
		}
		return textResult(out)
	case "servterm_status":
		names := namesArg(args["server"])
		out := []any{}
		for _, v := range s.Config.Servers {
			if len(names) > 0 && !contains(names, v.Name) {
				continue
			}
			if v.AgentURL == "" {
				out = append(out, map[string]any{"name": v.Name, "online": false, "error": "no agent configured"})
				continue
			}
			token, err := credential(v.TokenEnv, v.TokenFile)
			if err != nil {
				return nil, err
			}
			samples, err := agentclient.History(ctx, v.AgentURL, token, 5*time.Minute, 1)
			if err != nil {
				out = append(out, map[string]any{"name": v.Name, "online": false, "error": err.Error()})
				continue
			}
			if len(samples) == 0 {
				out = append(out, map[string]any{"name": v.Name, "online": false, "error": "no samples"})
				continue
			}
			out = append(out, map[string]any{"name": v.Name, "sample": samples[len(samples)-1]})
		}
		return textResult(out)
	case "servterm_history":
		name := stringArg(args, "server")
		v, err := findServer(s.Config, name)
		if err != nil {
			return nil, err
		}
		token, err := credential(v.TokenEnv, v.TokenFile)
		if err != nil {
			return nil, err
		}
		samples, err := agentclient.History(ctx, v.AgentURL, token, time.Duration(intArg(args, "minutes", 60))*time.Minute, intArg(args, "limit", 60))
		if err != nil {
			return nil, err
		}
		return textResult(map[string]any{"server": name, "samples": samples})
	case "servterm_stream":
		name := stringArg(args, "server")
		v, err := findServer(s.Config, name)
		if err != nil {
			return nil, err
		}
		token, err := credential(v.TokenEnv, v.TokenFile)
		if err != nil {
			return nil, err
		}
		stream, err := agentclient.Connect(ctx, v.AgentURL, token)
		if err != nil {
			return nil, err
		}
		defer stream.Close()
		n := intArg(args, "samples", 3)
		if n < 1 {
			n = 1
		}
		if n > 10 {
			n = 10
		}
		samples := []any{}
		for i := 0; i < n; i++ {
			w, e := stream.Read(ctx)
			if e != nil {
				return nil, e
			}
			samples = append(samples, w.Sample)
		}
		return textResult(map[string]any{"server": name, "samples": samples})
	case "servterm_list_desktops":
		out := []any{}
		for _, d := range s.Config.Desktops {
			out = append(out, map[string]any{"name": d.Name, "platform": d.Platform, "host": d.Host, "agent_url": d.AgentURL, "vnc_port": d.VNCPort})
		}
		return textResult(out)
	case "servterm_desktop_status":
		name := stringArg(args, "desktop")
		d, err := findDesktop(s.Config, name)
		if err != nil {
			return nil, err
		}
		token, err := credential(d.TokenEnv, d.TokenFile)
		if err != nil {
			return nil, err
		}
		return textResult(desktopclient.FetchStatus(ctx, *d, token))
	case "servterm_nvr_status":
		name := stringArg(args, "widget")
		for _, w := range s.Config.Widgets {
			if w.Name != name {
				continue
			}
			token, err := credential(w.TokenEnv, w.TokenFile)
			if err != nil {
				return nil, err
			}
			return textResult(widget.FetchNVR(ctx, w, token))
		}
		return nil, fmt.Errorf("unknown widget %q", name)
	default:
		return nil, fmt.Errorf("unknown tool %q", name)
	}
}
func textResult(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return map[string]any{"content": []any{map[string]any{"type": "text", "text": string(b)}}}, nil
}
func credential(env, path string) (string, error) {
	if env != "" {
		v := os.Getenv(env)
		if v == "" {
			return "", fmt.Errorf("credential environment variable %s is empty", env)
		}
		return v, nil
	}
	if path == "" {
		return "", errors.New("credential source is not configured")
	}
	b, err := os.ReadFile(config.ExpandHome(path))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
func stringArg(a map[string]any, k string) string { v, _ := a[k].(string); return v }
func intArg(a map[string]any, k string, d int) int {
	if v, ok := a[k].(float64); ok {
		return int(v)
	}
	return d
}
func namesArg(v any) []string {
	if s, ok := v.(string); ok && s != "" {
		return []string{s}
	}
	return nil
}
func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
func findServer(c config.Config, name string) (config.Server, error) {
	for _, v := range c.Servers {
		if v.Name == name {
			return v, nil
		}
	}
	return config.Server{}, fmt.Errorf("unknown server %q", name)
}
func findDesktop(c config.Config, name string) (*config.Desktop, error) {
	for i := range c.Desktops {
		if c.Desktops[i].Name == name {
			return &c.Desktops[i], nil
		}
	}
	return nil, fmt.Errorf("unknown desktop %q", name)
}
