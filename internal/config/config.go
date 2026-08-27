package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	RefreshInterval time.Duration `yaml:"-"`
	RefreshRaw      string        `yaml:"refresh_interval"`
	HistorySize     int           `yaml:"history_size"`
	SSH             SSHConfig     `yaml:"ssh"`
	Servers         []Server      `yaml:"servers"`
	Widgets         []Widget      `yaml:"widgets,omitempty"`
	Desktops        []Desktop     `yaml:"desktops,omitempty"`
}

type SSHConfig struct {
	ConnectTimeout        time.Duration `yaml:"-"`
	ConnectTimeoutRaw     string        `yaml:"connect_timeout"`
	CommandTimeout        time.Duration `yaml:"-"`
	CommandTimeoutRaw     string        `yaml:"command_timeout"`
	StrictHostKeyChecking *bool         `yaml:"strict_host_key_checking"`
}

type Server struct {
	Name         string   `yaml:"name"`
	Address      string   `yaml:"address"`
	User         string   `yaml:"user,omitempty"`
	Port         int      `yaml:"port,omitempty"`
	Transport    string   `yaml:"transport,omitempty"`
	Location     string   `yaml:"location,omitempty"`
	Tags         []string `yaml:"tags,omitempty"`
	IdentityFile string   `yaml:"identity_file,omitempty"`
	Disks        []string `yaml:"disks,omitempty"`
	AgentURL     string   `yaml:"agent_url,omitempty"`
	TokenEnv     string   `yaml:"token_env,omitempty"`
	TokenFile    string   `yaml:"token_file,omitempty"`
}

// Widget is a read-only external provider. Supported types are nvr,
// orchestrator, and cip.
type Widget struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
	// Host is the address of the server that runs this provider. It names
	// the one server whose detail view shows the widget. Leave it empty
	// when the endpoint already points at that server.
	Host      string `yaml:"host,omitempty"`
	Endpoint  string `yaml:"endpoint"`
	TokenEnv  string `yaml:"token_env,omitempty"`
	TokenFile string `yaml:"token_file,omitempty"`
}

// Desktop describes a managed graphical session. Credentials stay outside
// YAML; the agent endpoint is the authenticated control/status plane.
type Desktop struct {
	Name       string `yaml:"name"`
	Platform   string `yaml:"platform"`
	Host       string `yaml:"host"`
	VNCPort    int    `yaml:"vnc_port,omitempty"`
	AgentURL   string `yaml:"agent_url"`
	TokenEnv   string `yaml:"token_env,omitempty"`
	TokenFile  string `yaml:"token_file,omitempty"`
	SSHHost    string `yaml:"ssh_host,omitempty"`
	SSHUser    string `yaml:"ssh_user,omitempty"`
	SSHPort    int    `yaml:"ssh_port,omitempty"`
	Backend    string `yaml:"backend,omitempty"`
	RefreshFPS int    `yaml:"refresh_fps,omitempty"`
	Quality    string `yaml:"quality,omitempty"`
}

// HostAddress is the server address this widget belongs to. An explicit
// host wins. Otherwise the endpoint names the machine that runs the
// provider. It returns "" when neither gives a host, so a caller shows the
// widget on no server instead of on every server.
func (w Widget) HostAddress() string {
	if h := strings.TrimSpace(w.Host); h != "" {
		return h
	}
	u, err := url.Parse(strings.TrimSpace(w.Endpoint))
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func DefaultPath() string {
	if v := os.Getenv("SERVTERM_CONFIG"); v != "" {
		return v
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "servterm.yaml"
	}
	return filepath.Join(dir, "servterm", "config.yaml")
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var c Config
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("parse config: %w", err)
	}
	if c.RefreshRaw == "" {
		c.RefreshRaw = "3s"
	}
	if c.HistorySize == 0 {
		c.HistorySize = 60
	}
	if c.SSH.ConnectTimeoutRaw == "" {
		c.SSH.ConnectTimeoutRaw = "3s"
	}
	if c.SSH.CommandTimeoutRaw == "" {
		// Platform samplers such as top, ioreg and process enumeration can
		// legitimately take several seconds on a busy host.
		c.SSH.CommandTimeoutRaw = "15s"
	}
	if c.RefreshInterval, err = time.ParseDuration(c.RefreshRaw); err != nil {
		return c, fmt.Errorf("refresh_interval: %w", err)
	}
	if c.SSH.ConnectTimeout, err = time.ParseDuration(c.SSH.ConnectTimeoutRaw); err != nil {
		return c, fmt.Errorf("ssh.connect_timeout: %w", err)
	}
	if c.SSH.CommandTimeout, err = time.ParseDuration(c.SSH.CommandTimeoutRaw); err != nil {
		return c, fmt.Errorf("ssh.command_timeout: %w", err)
	}
	return c, c.Validate()
}

func (c Config) Validate() error {
	if len(c.Servers) == 0 {
		return errors.New("config must contain at least one server")
	}
	if c.RefreshInterval < time.Second {
		return errors.New("refresh_interval must be at least 1s")
	}
	if c.HistorySize < 10 || c.HistorySize > 600 {
		return errors.New("history_size must be between 10 and 600")
	}
	seen := map[string]bool{}
	for i, s := range c.Servers {
		p := fmt.Sprintf("servers[%d]", i)
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("%s.name is required", p)
		}
		if seen[s.Name] {
			return fmt.Errorf("duplicate server name %q", s.Name)
		}
		seen[s.Name] = true
		if strings.TrimSpace(s.Address) == "" {
			return fmt.Errorf("%s.address is required", p)
		}
		if s.Transport != "" && s.Transport != "ssh" && s.Transport != "local" {
			return fmt.Errorf("%s.transport must be ssh or local", p)
		}
		if s.AgentURL != "" && (s.TokenEnv == "") == (s.TokenFile == "") {
			return fmt.Errorf("%s requires exactly one of token_env or token_file with agent_url", p)
		}
		if s.Port < 0 || s.Port > 65535 {
			return fmt.Errorf("%s.port is invalid", p)
		}
	}
	for i, w := range c.Widgets {
		p := fmt.Sprintf("widgets[%d]", i)
		if strings.TrimSpace(w.Name) == "" || strings.TrimSpace(w.Endpoint) == "" {
			return fmt.Errorf("%s.name and endpoint are required", p)
		}
		if w.Type != "nvr" && w.Type != "orchestrator" && w.Type != "cip" {
			return fmt.Errorf("%s.type must be nvr, orchestrator, or cip", p)
		}
		if (w.TokenEnv == "") == (w.TokenFile == "") {
			return fmt.Errorf("%s requires exactly one of token_env or token_file", p)
		}
	}
	for i, d := range c.Desktops {
		p := fmt.Sprintf("desktops[%d]", i)
		if strings.TrimSpace(d.Name) == "" || strings.TrimSpace(d.Host) == "" || strings.TrimSpace(d.AgentURL) == "" {
			return fmt.Errorf("%s.name, host, and agent_url are required", p)
		}
		switch d.Platform {
		case "macos", "linux", "windows":
		default:
			return fmt.Errorf("%s.platform must be macos, linux, or windows", p)
		}
		if (d.TokenEnv == "") == (d.TokenFile == "") {
			return fmt.Errorf("%s requires exactly one of token_env or token_file", p)
		}
		if d.SSHPort < 0 || d.SSHPort > 65535 || d.VNCPort < 0 || d.VNCPort > 65535 {
			return fmt.Errorf("%s.ssh_port is invalid", p)
		}
		if d.RefreshFPS < 0 || d.RefreshFPS > 60 {
			return fmt.Errorf("%s.refresh_fps must be between 0 and 60", p)
		}
		if d.Quality != "" && d.Quality != "speed" && d.Quality != "balanced" && d.Quality != "quality" {
			return fmt.Errorf("%s.quality must be speed, balanced, or quality", p)
		}
	}
	return nil
}

func (s Server) IsLocal() bool { return s.Transport == "local" }

func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
