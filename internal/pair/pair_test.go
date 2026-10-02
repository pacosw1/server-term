package pair

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const statusSample = `{
  "Self": {"HostName": "Mac Mini", "DNSName": "mac-mini.tail1.ts.net.", "OS": "macOS",
           "Online": false, "TailscaleIPs": ["100.64.0.1", "fd7a::1"]},
  "Peer": {
    "a": {"HostName": "box", "DNSName": "box.tail1.ts.net.", "OS": "linux",
          "Online": true, "TailscaleIPs": ["fd7a::2", "100.64.0.2"]},
    "b": {"HostName": "pi", "DNSName": "pi.tail1.ts.net.", "OS": "linux",
          "Online": false, "TailscaleIPs": ["100.64.0.3"]},
    "c": {"HostName": "v6only", "DNSName": "v6.tail1.ts.net.", "Online": true,
          "TailscaleIPs": ["fd7a::4"]}
  }
}`

func TestParseStatusKeepsIPv4AndMarksSelfOnline(t *testing.T) {
	peers, err := ParseStatus([]byte(statusSample))
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 3 {
		t.Fatalf("want 3 peers (the IPv6-only one skipped), got %d", len(peers))
	}
	byName := map[string]Peer{}
	for _, p := range peers {
		byName[p.ShortName()] = p
	}
	if self := byName["mac-mini"]; !self.Self || !self.Online || self.IP != "100.64.0.1" {
		t.Fatalf("self = %+v", self)
	}
	if box := byName["box"]; box.IP != "100.64.0.2" {
		t.Fatalf("box picked %q, want the IPv4 address", box.IP)
	}
}

func TestShortNameFallsBackToHostName(t *testing.T) {
	if got := (Peer{HostName: "plain"}).ShortName(); got != "plain" {
		t.Fatalf("got %q", got)
	}
}

func TestDiscoverProbesOnlyOnlinePeersAndKeepsOrder(t *testing.T) {
	peers := []Peer{
		{HostName: "a", IP: "1", Online: true},
		{HostName: "b", IP: "2", Online: false},
		{HostName: "c", IP: "3", Online: true},
		{HostName: "d", IP: "4", Online: true},
	}
	probed := make(chan string, len(peers))
	probe := func(_ context.Context, ip string) (string, bool) {
		probed <- ip
		return "node-" + ip, ip != "4"
	}
	agents := Discover(context.Background(), peers, probe)
	close(probed)
	for ip := range probed {
		if ip == "2" {
			t.Fatal("an offline peer was probed")
		}
	}
	if len(agents) != 2 || agents[0].Peer.IP != "1" || agents[1].Peer.IP != "3" {
		t.Fatalf("agents = %+v", agents)
	}
	if agents[1].URL() != "http://3:7843" || agents[1].NodeID != "node-3" {
		t.Fatalf("agent = %+v url %s", agents[1], agents[1].URL())
	}
}

func TestPayloadMatchesTheAppImportFormat(t *testing.T) {
	p := Payload{
		Servers: []Server{
			{Name: "box", AgentURL: "http://100.64.0.2:7843", Token: "secret", SSHUser: "root"},
			{Name: "pi", AgentURL: "http://100.64.0.3:7843"},
		},
		Orchestrator: &Orchestrator{Name: "orch", Endpoint: "http://100.64.0.2:7844", Token: "t2"},
	}
	data, err := p.Encode()
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	first := raw["servers"].([]any)[0].(map[string]any)
	for _, key := range []string{"name", "agent_url", "token", "ssh_user"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("server lacks the key %q the app reads: %s", key, data)
		}
	}
	second := raw["servers"].([]any)[1].(map[string]any)
	if _, ok := second["token"]; ok {
		t.Fatal("an empty token must be left out, not sent as an empty string")
	}
	if got := p.Missing(); len(got) != 1 || got[0] != "pi" {
		t.Fatalf("missing = %v", got)
	}
}

func TestParseTokenReadsBareFileAndEnvFile(t *testing.T) {
	cases := map[string]string{
		"abc123\n": "abc123",
		"SERVTERM_LISTEN=100.64.0.1:7843\nSERVTERM_AGENT_TOKEN=deadbeef\n": "deadbeef",
		"SERVTERM_AGENT_TOKEN=\"quoted\"\n":                                "quoted",
		"SERVTERM_LISTEN=x\n# SERVTERM_AGENT_TOKEN= in a comment\n":        "",
	}
	for in, want := range cases {
		if got := ParseToken(in); got != want {
			t.Errorf("ParseToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLocalTokenTakesTheFirstReadableFile(t *testing.T) {
	dir := t.TempDir()
	env := filepath.Join(dir, "agent.env")
	if err := os.WriteFile(env, []byte("SERVTERM_AGENT_TOKEN=fromenv\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := LocalToken([]string{filepath.Join(dir, "missing.token"), env})
	if got != "fromenv" {
		t.Fatalf("got %q", got)
	}
	if LocalToken([]string{filepath.Join(dir, "none")}) != "" {
		t.Fatal("no file must give no token")
	}
}

func TestRenderQRHasAQuietZoneAndResetsColour(t *testing.T) {
	out, err := RenderQR(`{"servers":[]}`)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	// The top line is all quiet zone, so it holds light modules only.
	top := strings.TrimSuffix(strings.TrimPrefix(lines[0], "\x1b[97;40m"), "\x1b[0m")
	if strings.Trim(top, "█") != "" {
		t.Fatalf("the first line is not a light margin: %q", top)
	}
	for _, line := range lines {
		if !strings.HasSuffix(line, "\x1b[0m") {
			t.Fatal("a line leaves the terminal colour set")
		}
	}
}

func TestIsTailscaleIPCoversOnlyTheTailnetRange(t *testing.T) {
	cases := map[string]bool{
		"100.64.0.1":    true,
		"100.127.255.1": true,
		"100.128.0.1":   false,
		"192.168.1.10":  false,
		"box.ts.net":    false,
		"":              false,
	}
	for in, want := range cases {
		if got := IsTailscaleIP(in); got != want {
			t.Errorf("IsTailscaleIP(%q) = %v, want %v", in, got, want)
		}
	}
	if hostOf("root@100.64.0.1") != "100.64.0.1" || hostOf("100.64.0.1") != "100.64.0.1" {
		t.Fatal("hostOf must drop the user")
	}
}
