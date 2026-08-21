package devtools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/franciscosainzwilliams/server-term/internal/config"
)

type Tool struct{ ID, Description, Command, LinuxPackage, MacPackage string }

var Catalog = []Tool{
	{"git", "source control", "git", "git", "git"}, {"gh", "GitHub CLI", "gh", "gh", "gh"}, {"curl", "HTTP client", "curl", "curl", "curl"}, {"wget", "HTTP downloader", "wget", "wget", "wget"}, {"jq", "JSON processor", "jq", "jq", "jq"}, {"ripgrep", "fast code search", "rg", "ripgrep", "ripgrep"}, {"fd", "fast file finder", "fd", "fd-find", "fd"}, {"fzf", "fuzzy finder", "fzf", "fzf", "fzf"}, {"tmux", "terminal multiplexer", "tmux", "tmux", "tmux"}, {"htop", "process viewer", "htop", "htop", "htop"}, {"btop", "resource viewer", "btop", "btop", "btop"}, {"tree", "directory tree", "tree", "tree", "tree"}, {"unzip", "ZIP extractor", "unzip", "unzip", "unzip"}, {"python", "Python runtime", "python3", "python3", "python"}, {"node", "Node.js runtime", "node", "nodejs", "node"}, {"npm", "Node package manager", "npm", "npm", "npm"}, {"go", "Go toolchain", "go", "golang", "go"}, {"zsh", "Z shell", "zsh", "zsh", "zsh"}, {"neovim", "terminal editor", "nvim", "neovim", "neovim"}, {"shellcheck", "shell linter", "shellcheck", "shellcheck", "shellcheck"},
}

func Find(id string) (Tool, bool) {
	for _, t := range Catalog {
		if t.ID == id {
			return t, true
		}
	}
	return Tool{}, false
}
func Status(ctx context.Context, server config.Server) (map[string]bool, error) {
	ids := make([]string, len(Catalog))
	for i, t := range Catalog {
		ids[i] = t.Command
	}
	script := `for c in ` + strings.Join(ids, " ") + `; do if command -v "$c" >/dev/null 2>&1; then printf '%s\ttrue\n' "$c"; else printf '%s\tfalse\n' "$c"; fi; done`
	out, err := ssh(ctx, server, script)
	if err != nil {
		return nil, err
	}
	result := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		p := strings.SplitN(line, "\t", 2)
		if len(p) == 2 {
			result[p[0]] = p[1] == "true"
		}
	}
	return result, nil
}
func Install(ctx context.Context, server config.Server, id string, remove bool) (string, error) {
	t, ok := Find(id)
	if !ok {
		return "", fmt.Errorf("unknown dev tool %q", id)
	}
	pkg := t.LinuxPackage
	for _, tag := range server.Tags {
		if tag == "macos" || tag == "mac" {
			pkg = t.MacPackage
		}
	}
	action := "install"
	if remove {
		action = "remove"
	}
	script := fmt.Sprintf("set -eu; if command -v apt-get >/dev/null 2>&1; then sudo -n apt-get %s -y %s; elif command -v brew >/dev/null 2>&1; then brew %s %s; else echo 'no supported package manager' >&2; exit 2; fi", action, pkg, action, pkg)
	return ssh(ctx, server, script)
}
func ssh(ctx context.Context, s config.Server, script string) (string, error) {
	args := []string{"-o", "BatchMode=yes"}
	if s.Port != 0 {
		args = append(args, "-p", fmt.Sprint(s.Port))
	}
	target := s.Address
	if s.User != "" {
		target = s.User + "@" + target
	}
	// Pass the entire allowlisted script as one remote command. Adding a
	// separate `sh -lc` argument causes OpenSSH to re-tokenize loops under the
	// user's login shell (notably zsh), which breaks `for` scripts.
	args = append(args, target, script)
	b, err := exec.CommandContext(ctx, "ssh", args...).CombinedOutput()
	return string(b), err
}
