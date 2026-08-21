package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/franciscosainzwilliams/server-term/internal/agentclient"
	"github.com/franciscosainzwilliams/server-term/internal/config"
	"github.com/franciscosainzwilliams/server-term/internal/desktopclient"
	"github.com/franciscosainzwilliams/server-term/internal/devtools"
	"github.com/franciscosainzwilliams/server-term/internal/metrics"
	"github.com/franciscosainzwilliams/server-term/internal/widget"
)

type cliServer struct {
	Name   string          `json:"name"`
	Host   string          `json:"host"`
	Online bool            `json:"online"`
	Error  string          `json:"error,omitempty"`
	Sample *metrics.Sample `json:"sample,omitempty"`
}

type cliResult struct {
	SchemaVersion int         `json:"schema_version"`
	Command       string      `json:"command"`
	At            time.Time   `json:"at"`
	Servers       []cliServer `json:"servers,omitempty"`
}

func runCLI(ctx context.Context, cfg config.Config, args []string) error {
	command := args[0]
	if command == "watch" || command == "stream" {
		return cliWatch(ctx, cfg, args[1:])
	}
	if command == "ssh" || command == "shell" {
		return cliSSH(cfg, args[1:])
	}
	fs := flag.NewFlagSet("servterm "+command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "emit machine-readable JSON")
	host := fs.String("host", "", "server name (default: all servers)")
	minutes := fs.Int("minutes", 60, "history window in minutes")
	limit := fs.Int("limit", 60, "maximum samples")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *host == "" && len(fs.Args()) > 0 {
		*host = fs.Args()[0]
	}
	if command == "doctor" {
		return cliDoctor(ctx, cfg, *jsonOut)
	}
	if command == "widget" {
		return cliWidgets(ctx, cfg, *host, *jsonOut)
	}
	if command == "devtools" {
		return cliDevTools(ctx, cfg, fs.Args(), *jsonOut)
	}
	if command == "desktop" {
		return cliDesktops(ctx, cfg, fs.Args(), *jsonOut)
	}
	if command == "history" {
		return cliHistory(ctx, cfg, *host, *minutes, *limit, *jsonOut)
	}
	if command != "status" && command != "inspect" {
		return fmt.Errorf("unknown command %q", command)
	}
	result := collectLatest(ctx, cfg, *host)
	if *jsonOut {
		return printJSON(cliResult{SchemaVersion: 1, Command: command, At: time.Now(), Servers: result})
	}
	for _, item := range result {
		if !item.Online {
			fmt.Printf("%-20s OFFLINE  %s\n", item.Name, item.Error)
			continue
		}
		s := item.Sample
		fmt.Printf("%-20s online  CPU %5.1f%%  RAM %5.1f%%  power %s\n", item.Name, s.CPUPercent, metrics.Percent(s.MemTotal-s.MemAvailable, s.MemTotal), powerText(*s))
		if command == "inspect" {
			fmt.Printf("  host=%s os=%s kernel=%s uptime=%s latency=%s\n", s.Hostname, s.OS, s.Kernel, time.Duration(s.UptimeSeconds)*time.Second, s.Latency)
		}
	}
	if anyOffline(result) {
		return errors.New("one or more servers are offline")
	}
	return nil
}

func cliSSH(cfg config.Config, args []string) error {
	if len(args) < 1 {
		return errors.New("ssh requires a server name")
	}
	wanted := args[0]
	for _, server := range cfg.Servers {
		if server.Name != wanted {
			continue
		}
		sshArgs := []string{"-o", "BatchMode=yes"}
		if cfg.SSH.StrictHostKeyChecking != nil && !*cfg.SSH.StrictHostKeyChecking {
			sshArgs = append(sshArgs, "-o", "StrictHostKeyChecking=accept-new")
		}
		if server.Port != 0 {
			sshArgs = append(sshArgs, "-p", strconv.Itoa(server.Port))
		}
		if server.IdentityFile != "" {
			sshArgs = append(sshArgs, "-i", config.ExpandHome(server.IdentityFile))
		}
		target := server.Address
		if server.User != "" {
			target = server.User + "@" + target
		}
		sshArgs = append(sshArgs, target)
		if len(args) > 1 {
			sshArgs = append(sshArgs, args[1:]...)
		}
		cmd := exec.Command("ssh", sshArgs...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return fmt.Errorf("unknown server %q", wanted)
}

func cliDevTools(ctx context.Context, cfg config.Config, args []string, jsonOut bool) error {
	if containsArg(args, "--json") {
		jsonOut = true
	}
	if len(args) == 0 {
		return errors.New("devtools requires list, status, install, or uninstall")
	}
	action := args[0]
	if action == "list" {
		if jsonOut {
			return printJSON(map[string]any{"schema_version": 1, "tools": devtools.Catalog})
		}
		for _, t := range devtools.Catalog {
			fmt.Printf("%-14s %s\n", t.ID, t.Description)
		}
		return nil
	}
	if len(args) < 2 {
		return errors.New("devtools action requires a server name")
	}
	server, err := findServerConfig(cfg, args[1])
	if err != nil {
		return err
	}
	if action == "status" {
		statuses, err := devtools.Status(ctx, server)
		if err != nil {
			return err
		}
		if jsonOut {
			return printJSON(map[string]any{"schema_version": 1, "server": server.Name, "tools": statuses})
		}
		for _, t := range devtools.Catalog {
			state := "missing"
			if statuses[t.Command] {
				state = "installed"
			}
			fmt.Printf("%-14s %s\n", t.ID, state)
		}
		return nil
	}
	if action != "install" && action != "uninstall" && action != "remove" {
		return fmt.Errorf("unknown devtools action %q", action)
	}
	if len(args) < 3 {
		return errors.New("devtools install SERVER TOOL --yes")
	}
	if !containsArg(args, "--yes") {
		return errors.New("refusing package mutation without --yes")
	}
	out, err := devtools.Install(ctx, server, args[2], action != "install")
	if jsonOut {
		return printJSON(map[string]any{"schema_version": 1, "server": server.Name, "tool": args[2], "action": action, "output": out, "error": errorText(err)})
	}
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}
func findServerConfig(cfg config.Config, name string) (config.Server, error) {
	for _, s := range cfg.Servers {
		if s.Name == name {
			return s, nil
		}
	}
	return config.Server{}, fmt.Errorf("unknown server %q", name)
}
func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func collectLatest(ctx context.Context, cfg config.Config, wanted string) []cliServer {
	out := []cliServer{}
	for _, server := range cfg.Servers {
		if wanted != "" && server.Name != wanted {
			continue
		}
		item := cliServer{Name: server.Name, Host: server.Address}
		if server.AgentURL == "" {
			item.Error = "server has no agent_url"
			out = append(out, item)
			continue
		}
		token, err := tokenFor(server)
		if err == nil {
			var samples []metrics.Sample
			samples, err = agentclient.History(ctx, server.AgentURL, token, time.Minute*5, 1)
			if len(samples) > 0 {
				item.Sample = &samples[len(samples)-1]
				item.Online = item.Sample.Online
			}
		}
		if err != nil {
			item.Error = err.Error()
		}
		if item.Sample != nil && item.Sample.Error != "" {
			item.Error = item.Sample.Error
		}
		out = append(out, item)
	}
	return out
}

func cliHistory(ctx context.Context, cfg config.Config, wanted string, minutes, limit int, jsonOut bool) error {
	if minutes < 1 || limit < 1 {
		return errors.New("minutes and limit must be positive")
	}
	for _, server := range cfg.Servers {
		if wanted != "" && server.Name != wanted {
			continue
		}
		token, err := tokenFor(server)
		if err != nil {
			return fmt.Errorf("%s: %w", server.Name, err)
		}
		samples, err := agentclient.History(ctx, server.AgentURL, token, time.Duration(minutes)*time.Minute, limit)
		if err != nil {
			return fmt.Errorf("%s: %w", server.Name, err)
		}
		if jsonOut {
			if err := printJSON(map[string]any{"schema_version": 1, "command": "history", "server": server.Name, "samples": samples}); err != nil {
				return err
			}
		} else {
			for _, s := range samples {
				fmt.Printf("%s\t%s\tCPU %.1f%%\tRAM %.1f%%\n", s.At.Format(time.RFC3339), server.Name, s.CPUPercent, metrics.Percent(s.MemTotal-s.MemAvailable, s.MemTotal))
			}
		}
	}
	return nil
}

func cliWatch(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("servterm watch", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	host := fs.String("host", "", "server name")
	jsonOut := fs.Bool("json", true, "emit JSON lines")
	output := fs.String("output", "", "append JSON lines to this file instead of stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *host == "" && len(fs.Args()) > 0 {
		*host = fs.Args()[0]
	}
	var out io.Writer = os.Stdout
	var file *os.File
	if *output != "" {
		var err error
		file, err = os.OpenFile(config.ExpandHome(*output), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return err
		}
		defer file.Close()
		out = file
	}
	for _, server := range cfg.Servers {
		if *host != "" && server.Name != *host {
			continue
		}
		token, err := tokenFor(server)
		if err != nil {
			return fmt.Errorf("%s: %w", server.Name, err)
		}
		stream, err := agentclient.Connect(ctx, server.AgentURL, token)
		if err != nil {
			return fmt.Errorf("%s: %w", server.Name, err)
		}
		defer stream.Close()
		for {
			wire, err := stream.Read(ctx)
			if err != nil {
				return err
			}
			if *jsonOut {
				if err := writeJSON(out, map[string]any{"schema_version": 1, "server": server.Name, "sample": wire.Sample}); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(out, "%s CPU %.1f%%\n", server.Name, wire.Sample.CPUPercent)
			}
		}
	}
	return errors.New("no matching server")
}

func cliDoctor(ctx context.Context, cfg config.Config, jsonOut bool) error {
	result := collectLatest(ctx, cfg, "")
	if jsonOut {
		return printJSON(cliResult{SchemaVersion: 1, Command: "doctor", At: time.Now(), Servers: result})
	}
	for _, item := range result {
		if item.Online {
			fmt.Printf("OK   %-20s agent reachable\n", item.Name)
		} else {
			fmt.Printf("FAIL %-20s %s\n", item.Name, item.Error)
		}
	}
	if anyOffline(result) {
		return errors.New("doctor found failures")
	}
	return nil
}

func cliWidgets(ctx context.Context, cfg config.Config, wanted string, jsonOut bool) error {
	if len(cfg.Widgets) == 0 {
		return errors.New("no widgets configured")
	}
	failed := false
	for _, provider := range cfg.Widgets {
		if wanted != "" && provider.Name != wanted {
			continue
		}
		token, err := tokenForWidget(provider)
		if err != nil {
			return fmt.Errorf("%s: %w", provider.Name, err)
		}
		snapshot := widget.FetchNVR(ctx, provider, token)
		if snapshot.Error != "" {
			failed = true
		}
		if jsonOut {
			if err := printJSON(snapshot); err != nil {
				return err
			}
		} else {
			fmt.Printf("%-20s %s  streams %d/%d  CPU %.1f%%\n", snapshot.Name, map[bool]string{true: "healthy", false: "degraded"}[snapshot.Healthy], snapshot.LiveStreams, snapshot.TotalStreams, snapshot.CPUPercent)
		}
	}
	if wanted != "" {
		for _, provider := range cfg.Widgets {
			if provider.Name == wanted {
				if failed {
					return errors.New("widget failed")
				}
				return nil
			}
		}
		return errors.New("no matching widget")
	}
	if failed {
		return errors.New("one or more widgets failed")
	}
	return nil
}

func cliDesktops(ctx context.Context, cfg config.Config, args []string, jsonOut bool) error {
	if len(args) == 0 {
		return errors.New("desktop requires list or doctor")
	}
	action := args[0]
	wanted := ""
	if len(args) > 1 && args[1] != "--json" {
		wanted = args[1]
	}
	if len(args) > 1 && args[1] == "--json" {
		jsonOut = true
	}
	if action != "list" && action != "doctor" && action != "connect" && action != "key" && action != "click" {
		if action != "screenshot" {
			return fmt.Errorf("unsupported desktop action %q", action)
		}
	}
	if action == "screenshot" && len(args) < 3 {
		return errors.New("desktop screenshot NAME OUTPUT.png")
	}
	if (action == "key" && len(args) < 3) || (action == "click" && len(args) < 4) {
		return errors.New("desktop key NAME COMBO or desktop click NAME X Y")
	}
	if action == "key" || action == "click" {
		wanted = args[1]
		for _, desktop := range cfg.Desktops {
			if desktop.Name != wanted {
				continue
			}
			token, err := tokenForDesktop(desktop)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			if action == "key" {
				err = desktopclient.SendKey(ctx, desktop, token, args[2])
			} else {
				x, _ := strconv.Atoi(args[2])
				y, _ := strconv.Atoi(args[3])
				err = desktopclient.Click(ctx, desktop, token, x, y)
			}
			if err != nil {
				return err
			}
			if jsonOut {
				return printJSON(map[string]any{"schema_version": 1, "desktop": wanted, "action": action, "ok": true})
			}
			fmt.Println("ok")
			return nil
		}
		return errors.New("no matching desktop")
	}
	if action == "screenshot" {
		wanted = args[1]
		for _, desktop := range cfg.Desktops {
			if desktop.Name != wanted {
				continue
			}
			token, err := tokenForDesktop(desktop)
			if err != nil {
				return err
			}
			data, err := desktopclient.FetchScreenshot(ctx, desktop, token)
			if err != nil {
				return err
			}
			if err := os.WriteFile(config.ExpandHome(args[2]), data, 0600); err != nil {
				return err
			}
			if jsonOut {
				return printJSON(map[string]any{"schema_version": 1, "desktop": wanted, "output": args[2], "captured": true})
			}
			fmt.Printf("captured %s\n", args[2])
			return nil
		}
		return errors.New("no matching desktop")
	}
	failed := false
	for _, desktop := range cfg.Desktops {
		if wanted != "" && desktop.Name != wanted {
			continue
		}
		if action == "list" {
			if jsonOut {
				if err := printJSON(map[string]any{"schema_version": 1, "desktop": desktop}); err != nil {
					return err
				}
			} else {
				fmt.Printf("%-20s %-8s %s agent=%s\n", desktop.Name, desktop.Platform, desktop.Host, desktop.AgentURL)
			}
			continue
		}
		if action == "connect" {
			port := desktop.VNCPort
			if port == 0 {
				port = 5900
			}
			uri := fmt.Sprintf("vnc://%s:%d", desktop.Host, port)
			var cmd *exec.Cmd
			if runtime.GOOS == "darwin" {
				cmd = exec.Command("open", uri)
			} else {
				cmd = exec.Command("xdg-open", uri)
			}
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("open desktop %s: %w", desktop.Name, err)
			}
			if jsonOut {
				return printJSON(map[string]any{"schema_version": 1, "desktop": desktop.Name, "uri": uri, "opened": true})
			}
			fmt.Printf("opened %s\n", uri)
			return nil
		}
		token, err := tokenForDesktop(desktop)
		if err != nil {
			if jsonOut {
				_ = printJSON(map[string]any{"schema_version": 1, "desktop": desktop.Name, "error": err.Error()})
			}
			return fmt.Errorf("%s: %w", desktop.Name, err)
		}
		status := desktopclient.FetchStatus(ctx, desktop, token)
		if !status.Healthy() {
			failed = true
		}
		if jsonOut {
			if err := printJSON(map[string]any{"schema_version": 1, "desktop": desktop.Name, "status": status}); err != nil {
				return err
			}
		} else {
			state := "offline"
			if status.Healthy() && hasCapability(status.Capabilities, "screenshot") {
				state = "ready"
			} else if status.Healthy() {
				state = "needs-permission"
			}
			fmt.Printf("%-20s %-8s %-7s backend=%s\n", desktop.Name, state, desktop.Platform, status.Backend)
		}
	}
	if wanted != "" {
		for _, d := range cfg.Desktops {
			if d.Name == wanted {
				if failed {
					return errors.New("desktop check failed")
				}
				return nil
			}
		}
		return errors.New("no matching desktop")
	}
	if len(cfg.Desktops) == 0 {
		return errors.New("no desktops configured")
	}
	if failed {
		return errors.New("one or more desktops failed")
	}
	return nil
}

func hasCapability(capabilities []string, wanted string) bool {
	for _, capability := range capabilities {
		if capability == wanted {
			return true
		}
	}
	return false
}

func tokenFor(server config.Server) (string, error) {
	if server.TokenEnv != "" {
		if token := os.Getenv(server.TokenEnv); token != "" {
			return token, nil
		}
		return "", fmt.Errorf("token environment variable %s is empty", server.TokenEnv)
	}
	if server.TokenFile == "" {
		return "", errors.New("token_file or token_env is required")
	}
	b, err := os.ReadFile(config.ExpandHome(server.TokenFile))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
func tokenForWidget(provider config.Widget) (string, error) {
	if provider.TokenEnv != "" {
		if token := os.Getenv(provider.TokenEnv); token != "" {
			return token, nil
		}
		return "", fmt.Errorf("token environment variable %s is empty", provider.TokenEnv)
	}
	b, err := os.ReadFile(config.ExpandHome(provider.TokenFile))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
func tokenForDesktop(desktop config.Desktop) (string, error) {
	if desktop.TokenEnv != "" {
		if token := os.Getenv(desktop.TokenEnv); token != "" {
			return token, nil
		}
		return "", fmt.Errorf("token environment variable %s is empty", desktop.TokenEnv)
	}
	b, err := os.ReadFile(config.ExpandHome(desktop.TokenFile))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
func writeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
func anyOffline(items []cliServer) bool {
	for _, item := range items {
		if !item.Online {
			return true
		}
	}
	return false
}
func powerText(s metrics.Sample) string {
	if !s.PowerKnown {
		return "n/a"
	}
	return fmt.Sprintf("%.1fW", s.PowerWatts)
}
