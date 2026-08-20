package desktopclient

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/franciscosainzwilliams/server-term/internal/config"
)

type localTunnel struct {
	process *exec.Cmd
	port    int
}

func (t *localTunnel) Close() {
	if t != nil && t.process != nil && t.process.Process != nil {
		_ = t.process.Process.Kill()
	}
}

func endpoint(ctx context.Context, desktop config.Desktop) (string, *localTunnel, error) {
	if desktop.SSHUser == "" || (!strings.HasPrefix(desktop.AgentURL, "http://127") && !strings.HasPrefix(desktop.AgentURL, "http://localhost")) {
		return desktop.AgentURL, nil, nil
	}
	sshHost := desktop.SSHHost
	if sshHost == "" {
		sshHost = desktop.Host
	}
	port := desktop.SSHPort
	if port == 0 {
		port = 22
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	localPort := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	cmd := exec.CommandContext(ctx, "ssh", "-N", "-o", "BatchMode=yes", "-o", "ExitOnForwardFailure=yes", "-p", strconv.Itoa(port), "-L", fmt.Sprintf("%d:%s:7850", localPort, desktop.Host), fmt.Sprintf("%s@%s", desktop.SSHUser, sshHost))
	if err := cmd.Start(); err != nil {
		return "", nil, err
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), 250*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return fmt.Sprintf("http://127.0.0.1:%d", localPort), &localTunnel{cmd, localPort}, nil
		}
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return "", nil, ctx.Err()
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
	_ = cmd.Process.Kill()
	return "", nil, fmt.Errorf("SSH desktop tunnel did not open")
}
