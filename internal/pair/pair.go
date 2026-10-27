// Package pair finds the servterm agents on the tailnet and builds the one
// setup payload that the iPhone app imports from a QR code. The payload is
// the same JSON as the app's servterm-import.json file.
package pair

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

// AgentPort is the port that every servterm agent listens on.
const AgentPort = 7843

// Peer is one machine on the tailnet.
type Peer struct {
	HostName string
	DNSName  string
	IP       string
	OS       string
	Online   bool
	Self     bool
}

// ShortName is the first label of the MagicDNS name, which is the name the
// user sees in the Tailscale admin, cleaned of quotes and spaces. It falls
// back to the host name.
func (p Peer) ShortName() string {
	if name, _, ok := strings.Cut(strings.TrimSuffix(p.DNSName, "."), "."); ok && name != "" {
		return name
	}
	return p.HostName
}

type statusJSON struct {
	Self *peerJSON           `json:"Self"`
	Peer map[string]peerJSON `json:"Peer"`
}

type peerJSON struct {
	HostName     string   `json:"HostName"`
	DNSName      string   `json:"DNSName"`
	OS           string   `json:"OS"`
	Online       bool     `json:"Online"`
	TailscaleIPs []string `json:"TailscaleIPs"`
}

// ParseStatus reads the output of `tailscale status --json`. It keeps the
// IPv4 address of each machine and skips a machine without one.
func ParseStatus(data []byte) ([]Peer, error) {
	var s statusJSON
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("read tailscale status: %w", err)
	}
	var peers []Peer
	add := func(p peerJSON, self bool) {
		ip := ipv4(p.TailscaleIPs)
		if ip == "" {
			return
		}
		peers = append(peers, Peer{
			HostName: p.HostName, DNSName: p.DNSName, IP: ip, OS: p.OS,
			Online: p.Online || self, Self: self,
		})
	}
	if s.Self != nil {
		add(*s.Self, true)
	}
	for _, p := range s.Peer {
		add(p, false)
	}
	sort.Slice(peers, func(i, j int) bool { return peers[i].ShortName() < peers[j].ShortName() })
	return peers, nil
}

func ipv4(ips []string) string {
	for _, s := range ips {
		if ip := net.ParseIP(s); ip != nil && ip.To4() != nil {
			return s
		}
	}
	return ""
}

// TailscaleStatus runs the tailscale CLI. On macOS the CLI often lives only
// inside the app bundle, so it looks there when the PATH has none.
func TailscaleStatus(ctx context.Context) ([]byte, error) {
	bin, err := exec.LookPath("tailscale")
	if err != nil {
		const bundled = "/Applications/Tailscale.app/Contents/MacOS/Tailscale"
		if _, statErr := os.Stat(bundled); statErr != nil {
			return nil, errors.New("tailscale CLI not found; is Tailscale installed and running?")
		}
		bin = bundled
	}
	out, err := exec.CommandContext(ctx, bin, "status", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("tailscale status: %w", err)
	}
	return out, nil
}

// Probe asks one address whether a servterm agent answers there. The status
// route needs no token, so a probe never sends one.
type Probe func(ctx context.Context, ip string) (nodeID string, ok bool)

// HTTPProbe is the real probe.
func HTTPProbe(timeout time.Duration) Probe {
	client := &http.Client{Timeout: timeout}
	return func(ctx context.Context, ip string) (string, bool) {
		url := fmt.Sprintf("http://%s/v1/status", net.JoinHostPort(ip, fmt.Sprint(AgentPort)))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", false
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", false
		}
		defer resp.Body.Close()
		var body struct {
			Service string `json:"service"`
			NodeID  string `json:"node_id"`
		}
		if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&body) != nil {
			return "", false
		}
		return body.NodeID, body.Service == "servterm-agent"
	}
}

// Agent is one machine where an agent answered.
type Agent struct {
	Peer   Peer
	NodeID string
}

// URL is the agent address the app reads.
func (a Agent) URL() string {
	return fmt.Sprintf("http://%s", net.JoinHostPort(a.Peer.IP, fmt.Sprint(AgentPort)))
}

// Discover probes every online machine at once and returns the ones that
// run an agent, in the order of the peers.
func Discover(ctx context.Context, peers []Peer, probe Probe) []Agent {
	found := make([]*Agent, len(peers))
	var wg sync.WaitGroup
	for i, p := range peers {
		if !p.Online {
			continue
		}
		wg.Add(1)
		go func(i int, p Peer) {
			defer wg.Done()
			if node, ok := probe(ctx, p.IP); ok {
				found[i] = &Agent{Peer: p, NodeID: node}
			}
		}(i, p)
	}
	wg.Wait()
	var agents []Agent
	for _, a := range found {
		if a != nil {
			agents = append(agents, *a)
		}
	}
	return agents
}

// Server is one entry of the payload. The keys match the app's import file.
type Server struct {
	Name     string `json:"name"`
	AgentURL string `json:"agent_url"`
	Location string `json:"location,omitempty"`
	Token    string `json:"token,omitempty"`
	SSHUser  string `json:"ssh_user,omitempty"`
}

// Orchestrator is the optional orchestrator entry of the payload.
type Orchestrator struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	Token    string `json:"token"`
}

// Payload is the whole setup that the app imports.
type Payload struct {
	Servers      []Server      `json:"servers"`
	Orchestrator *Orchestrator `json:"orchestrator,omitempty"`
}

// Encode is the compact JSON that goes in the QR code.
func (p Payload) Encode() ([]byte, error) {
	return json.Marshal(p)
}

// Missing names the servers that carry no token. The app lists them, but
// cannot read them until the user adds the token.
func (p Payload) Missing() []string {
	var names []string
	for _, s := range p.Servers {
		if s.Token == "" {
			names = append(names, s.Name)
		}
	}
	return names
}
