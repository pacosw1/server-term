package pair

import (
	"bufio"
	"context"
	"errors"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// tokenKey is the line of the Linux agent.env file that holds the token.
const tokenKey = "SERVTERM_AGENT_TOKEN="

// LocalTokenPaths are the places where the installers leave the token of
// the agent on this machine: the macOS state folder, then the Linux
// environment file.
func LocalTokenPaths() []string {
	var paths []string
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, "Library", "Application Support", "servterm", "agent.token"))
	}
	return append(paths, "/etc/servterm/agent.env")
}

// LocalToken reads the token of the agent on this machine. It returns ""
// when no file is readable, which is normal on a machine without an agent.
func LocalToken(paths []string) string {
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if token := ParseToken(string(data)); token != "" {
			return token
		}
	}
	return ""
}

// ParseToken reads a bare token file or an agent.env file.
func ParseToken(text string) string {
	if !strings.Contains(text, tokenKey) {
		return strings.TrimSpace(text)
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if value, ok := strings.CutPrefix(line, tokenKey); ok {
			return strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return ""
}

// remoteTokenScript prints the agent token of a macOS or a Linux host. It
// uses sudo -n so that it never waits on a password prompt.
const remoteTokenScript = `cat "$HOME/Library/Application Support/servterm/agent.token" 2>/dev/null ||
cat /etc/servterm/agent.env 2>/dev/null ||
sudo -n cat /etc/servterm/agent.env 2>/dev/null`

// SSHToken reads the agent token of a remote host over OpenSSH. BatchMode
// makes it fail at once, rather than ask, for a password. The token goes
// into memory only.
//
// A Tailscale address is authenticated by WireGuard: only that node can
// answer on it. So for such an address a host key seen for the first time
// is accepted and recorded. A key that changed is still refused.
func SSHToken(ctx context.Context, target string) (string, error) {
	args := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=5"}
	if IsTailscaleIP(hostOf(target)) {
		args = append(args, "-o", "StrictHostKeyChecking=accept-new")
	}
	cmd := exec.CommandContext(ctx, "ssh", append(args, target, remoteTokenScript)...)
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			return "", errors.New(strings.TrimSpace(lastLine(string(exit.Stderr))))
		}
		return "", err
	}
	token := ParseToken(string(out))
	if token == "" {
		return "", errors.New("no agent token on the host")
	}
	return token, nil
}

// tailnetRange is the CGNAT block that Tailscale assigns addresses from.
var tailnetRange = netip.MustParsePrefix("100.64.0.0/10")

// IsTailscaleIP reports whether host is an address inside the tailnet range.
func IsTailscaleIP(host string) bool {
	addr, err := netip.ParseAddr(host)
	return err == nil && tailnetRange.Contains(addr)
}

func hostOf(target string) string {
	if _, host, ok := strings.Cut(target, "@"); ok {
		return host
	}
	return target
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return lines[len(lines)-1]
}
