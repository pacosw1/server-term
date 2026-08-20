package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/franciscosainzwilliams/server-term/internal/config"
	"github.com/franciscosainzwilliams/server-term/internal/ui"
)

func isCLICommand(command string) bool {
	switch command {
	case "status", "inspect", "history", "watch", "stream", "doctor", "widget", "desktop":
		return true
	}
	return false
}

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "servterm:", err)
		os.Exit(1)
	}
}
func run() error {
	fs := flag.NewFlagSet("servterm", flag.ContinueOnError)
	path := fs.String("config", config.DefaultPath(), "path to YAML config")
	showVersion := fs.Bool("version", false, "print version")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if *showVersion {
		fmt.Println("servterm " + version)
		return nil
	}
	args := fs.Args()
	if len(args) > 0 && args[0] == "init" {
		return initConfig(*path)
	}
	cfg, err := config.Load(*path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("config not found at %s (run `servterm init`)", *path)
		}
		return err
	}
	if len(args) > 0 && args[0] == "validate" {
		fmt.Printf("✓ %s: %d servers, refresh every %s\n", *path, len(cfg.Servers), cfg.RefreshInterval)
		return nil
	}
	if len(args) > 0 {
		if isCLICommand(args[0]) {
			return runCLI(context.Background(), cfg, args)
		}
		return fmt.Errorf("unknown command %q", args[0])
	}
	p := tea.NewProgram(ui.New(cfg), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}
func initConfig(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("refusing to overwrite existing %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	body := strings.TrimSpace(exampleConfig) + "\n"
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		return err
	}
	fmt.Println("Created", path)
	fmt.Println("Edit it, then run: servterm validate")
	return nil
}

const exampleConfig = `refresh_interval: 3s
history_size: 60
ssh:
  connect_timeout: 3s
  command_timeout: 15s
  strict_host_key_checking: true
servers:
  - name: this-machine
    address: localhost
    transport: local
    location: Local
    tags: [development]
  # - name: production
  #   address: server.your-tailnet.ts.net
  #   user: deploy
  #   location: Monterrey
  #   tags: [production, web]
  #   agent_url: http://100.64.0.10:7843
  #   token_file: ~/.config/servterm/tokens/production
`
