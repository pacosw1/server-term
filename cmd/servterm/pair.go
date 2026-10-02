package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/franciscosainzwilliams/server-term/internal/config"
	"github.com/franciscosainzwilliams/server-term/internal/pair"
)

// cliPair finds every agent on the tailnet and shows one QR code that the
// iPhone app scans to add them all. It needs no inventory; when one exists
// it supplies names, locations, shell accounts and tokens.
func cliPair(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("pair", flag.ContinueOnError)
	useSSH := fs.Bool("ssh", false, "read a missing agent token from the host over ssh")
	sshUser := fs.String("ssh-user", "", "shell account for a server that the inventory does not name")
	file := fs.String("file", "", "write the setup JSON to this file (mode 0600) instead of showing a QR code")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	status, err := pair.TailscaleStatus(ctx)
	if err != nil {
		return err
	}
	peers, err := pair.ParseStatus(status)
	if err != nil {
		return err
	}
	agents := pair.Discover(ctx, peers, pair.HTTPProbe(3*time.Second))
	if len(agents) == 0 {
		return fmt.Errorf("no servterm agent answered on port %d of any online tailnet machine", pair.AgentPort)
	}

	var payload pair.Payload
	var lines [][3]string
	fmt.Fprintf(os.Stderr, "Found %d agents on your tailnet:\n", len(agents))
	for _, agent := range agents {
		server, known := matchServer(cfg, agent)
		entry := pair.Server{Name: agent.Peer.ShortName(), AgentURL: agent.URL(), SSHUser: *sshUser}
		token, source := "", ""
		if known {
			entry.Name, entry.Location = server.Name, server.Location
			if server.User != "" {
				entry.SSHUser = server.User
			}
			if server.AgentURL != "" {
				if t, err := tokenFor(server); err == nil {
					token, source = t, "inventory"
				}
			}
		}
		if token == "" && agent.Peer.Self {
			if t := pair.LocalToken(pair.LocalTokenPaths()); t != "" {
				token, source = t, "this machine"
			}
		}
		if token == "" && *useSSH {
			target := sshTarget(server, known, agent.Peer, entry.SSHUser)
			if t, err := pair.SSHToken(ctx, target); err == nil {
				token, source = t, "ssh"
			} else {
				source = "ssh failed: " + err.Error()
			}
		}
		entry.Token = token
		payload.Servers = append(payload.Servers, entry)
		mark := "✓"
		if token == "" {
			mark = "✗"
			if source == "" {
				source = "no token (try --ssh)"
			}
		}
		lines = append(lines, [3]string{mark + " " + entry.Name, agent.URL(), source})
	}
	width := 0
	for _, l := range lines {
		width = max(width, len([]rune(l[0])))
	}
	for _, l := range lines {
		fmt.Fprintf(os.Stderr, "  %-*s  %-26s %s\n", width, l[0], l[1], l[2])
	}
	payload.Orchestrator = orchestratorEntry(cfg)

	data, err := payload.Encode()
	if err != nil {
		return err
	}
	if *file != "" {
		if err := os.WriteFile(*file, data, 0o600); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Wrote %s. It holds your tokens; delete it after the import.\n", *file)
		return nil
	}
	code, err := pair.RenderQR(string(data))
	if err != nil {
		return fmt.Errorf("the setup is too large for one QR code (%d bytes); use --file", len(data))
	}
	fmt.Fprintln(os.Stderr)
	fmt.Print(code)
	fmt.Fprintln(os.Stderr, "\nIn the app: tap the gear, then Scan pairing code.")
	fmt.Fprintln(os.Stderr, "The code holds your tokens. Clear the terminal after you scan it.")
	if missing := payload.Missing(); len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "No token for: %s. The app lists them but cannot read them.\n", strings.Join(missing, ", "))
	}
	return nil
}

// matchServer finds the inventory entry of an agent by its address.
func matchServer(cfg config.Config, agent pair.Agent) (config.Server, bool) {
	names := []string{agent.Peer.IP, agent.Peer.ShortName(), strings.TrimSuffix(agent.Peer.DNSName, ".")}
	for _, server := range cfg.Servers {
		hosts := []string{server.Address}
		if u, err := url.Parse(server.AgentURL); err == nil && u.Hostname() != "" {
			hosts = append(hosts, u.Hostname())
		}
		for _, host := range hosts {
			for _, name := range names {
				if host != "" && strings.EqualFold(host, name) {
					return server, true
				}
			}
		}
	}
	return config.Server{}, false
}

// sshTarget is the ssh destination: the inventory host when there is one,
// so ~/.ssh/config applies, else the MagicDNS name.
func sshTarget(server config.Server, known bool, peer pair.Peer, user string) string {
	host := strings.TrimSuffix(peer.DNSName, ".")
	if known && server.Address != "" {
		host, user = server.Address, server.User
	}
	if host == "" {
		host = peer.IP
	}
	if user != "" {
		return user + "@" + host
	}
	return host
}

// orchestratorEntry carries the first orchestrator widget that has a token.
func orchestratorEntry(cfg config.Config) *pair.Orchestrator {
	for _, w := range cfg.Widgets {
		if w.Type != "orchestrator" {
			continue
		}
		if token, err := tokenForWidget(w); err == nil && token != "" {
			return &pair.Orchestrator{Name: w.Name, Endpoint: w.Endpoint, Token: token}
		}
	}
	return nil
}
