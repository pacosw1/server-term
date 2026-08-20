# servterm

`servterm` is a friendly, live terminal dashboard for a small fleet of Linux servers. It reads a local YAML inventory and collects metrics through your existing OpenSSH setup, so Tailscale hostnames, `~/.ssh/config`, ssh-agent, hardware keys, and jump hosts continue to work normally.

For high-fidelity mode, a small `servterm-agent` samples once per second, stores seven days in local SQLite/WAL, and streams over an authenticated WebSocket bound to the server's Tailscale address. The original one-shot SSH collector remains available as bootstrap and fallback.

## What it shows

- Fleet overview: online state, SSH latency, location, CPU, memory, root disk, network throughput, and CPU history
- Server detail: host/OS/kernel, uptime, cores, load, memory/swap, platform pressure, power/battery telemetry, filesystems, and SSD/HDD devices
- Live, independently timed collection so one offline server does not block the fleet
- Responsive compact layout for narrow terminals
- Local transport for monitoring the Linux host running `servterm`
- Persistent agent mode with SQLite history and direct Tailscale streaming
- Per-core CPU grid, top-process view, and sanitized per-job GitHub runner accounting

The Network detail tab also reports the active/default interface, connection
type (Ethernet or Wi-Fi where the OS exposes it), negotiated link speed, live
throughput, and cumulative RX/TX errors and drops. It deliberately does not run
an automatic Internet speed test: that would generate traffic and distort the
same throughput measurements. An explicit on-demand speed-test command can be
added separately.

## Build and run

Go 1.24 or newer is required to build. The resulting binary has no Go runtime dependency.

```sh
make build
./bin/servterm init
$EDITOR ~/.config/servterm/config.yaml
./bin/servterm validate
./bin/servterm
```

## CLI and automation

The same binary has non-interactive commands for agents and scripts. Put
`--config PATH` before the command when using a non-default inventory:

```sh
servterm --config ~/.config/servterm/config.yaml status --json
servterm inspect --host office-nvrd --json
servterm history --host hetzner-32cpu --minutes 60 --limit 60 --json
servterm doctor --json
```

`status`, `inspect`, `history`, and `doctor` return schema-versioned JSON with
`--json`. A failing host produces a non-zero exit status. `watch` (also named
`stream`) is a long-running newline-delimited JSON feed suitable for another
agent or a supervisor:

```sh
servterm watch --host office-nvrd --output ~/.cache/servterm/office-nvrd.jsonl &
```

The stream process is intentionally foreground-friendly: use your service
manager, `nohup`, or a shell supervisor to run it in the background and restart
it. The output file is append-only and mode `0600`; each line has
`schema_version`, `server`, and `sample` fields.

Desktop frames use a persistent agent WebSocket/RFB session while the DESKTOP
tab is active; they are not fetched by reconnecting VNC for every frame. Set
`refresh_fps: 60` for the target rate and choose `quality: speed|balanced|quality`.
Kitty and iTerm2 terminals receive inline GPU-rendered images; other terminals
fall back to ANSI half-blocks. `SERVTERM_DESKTOP_RENDER=kitty|iterm2|blocks`
overrides terminal detection.

### NVR widget provider

Add a read-only NVR stats provider to the inventory:

```yaml
widgets:
  - name: office-nvr
    type: nvr
    endpoint: http://127.0.0.1:8085
    token_file: ~/.config/servterm/widgets/office-nvr
```

Query its normalized snapshot with `servterm widget --host office-nvr --json`.
The provider calls only authenticated `GET /api/stats`; it does not execute
plugin code or expose NVR control actions. It reports stream liveness, drops,
disk/archive usage, and nvrd CPU/RSS.

### Desktop agents

Desktop inventory is separate from host metrics and keeps control credentials
outside YAML:

```yaml
desktops:
  - name: office-mac
    platform: macos
    host: 100.89.120.115
    vnc_port: 5900
    agent_url: http://100.89.120.115:7850
    token_file: ~/.config/servterm/desktop/office-mac
    ssh_user: paco
```

The initial desktop-agent binary exposes authenticated capability/status
endpoints and fails closed when no native capture backend is installed:

```sh
make build-desktop-agent
servterm desktop list
servterm desktop doctor office-mac --json
servterm desktop connect office-mac
servterm desktop screenshot office-mac /tmp/office-mac.png
```

Screen capture/input backends remain platform-specific. The installer will
probe and install only an explicitly supported backend; it will not execute
arbitrary shell commands or put VNC passwords in the inventory.

Installers are provided for the agent control plane:

```sh
sudo deploy/install-desktop-agent-linux.sh \
  --binary ./bin/servterm-desktop-agent-linux-amd64 \
  --listen TAILSCALE_IP:7850 --node linux-box --platform linux \
  --token /secure/desktop-agent.token \
  --vnc-password /secure/vnc.password
```

On macOS use `deploy/install-macos-desktop-agent-user.sh`. Native screen
sharing/Accessibility permissions and the VNC backend remain explicit host
setup steps; the installer never silently grants those privileges.

To try the checked-in example on Linux:

```sh
go run ./cmd/servterm --config servterm.example.yaml
```

Use `--config PATH` or `SERVTERM_CONFIG` for another inventory. Files created by `init` use mode `0600`.

## Configuration

```yaml
refresh_interval: 3s
history_size: 60
ssh:
  connect_timeout: 3s
  command_timeout: 15s
  strict_host_key_checking: true
servers:
  - name: web-01
    address: web-01.your-tailnet.ts.net
    user: deploy
    port: 22
    location: Monterrey
    tags: [production, web]
    # identity_file: ~/.ssh/id_ed25519
    # disks: [/, /data]
    agent_url: http://100.64.0.10:7843
    token_file: ~/.config/servterm/tokens/web-01
```

SSH is the default transport. Set `transport: local` to inspect the Linux machine running `servterm`. Keep strict host-key checking enabled for normal use; setting it to false uses OpenSSH `accept-new`, which still rejects changed keys.

By default all mounted filesystems are included. Set `disks` to a list of mount paths when you only want important volumes in the detail view.

`agent_url` switches a host from periodic SSH collection to a persistent stream. Keep the bearer token outside YAML in a mode-`0600` file. Alternatively, use `token_env` instead of `token_file` to name an environment variable:

```sh
export SERVTERM_WEB_01_TOKEN='the-per-server-token' # only with token_env
servterm
```

## Agent deployment

Build the static Linux x86-64 agent with `make build-agent-linux`. Install it as `/usr/local/bin/servterm-agent`, create a locked-down `servterm` system user, and use [deploy/servterm-agent.service](deploy/servterm-agent.service). The environment file at `/etc/servterm/agent.env` must be owned by root with mode `0600`.

### macOS agent

Build Apple Silicon and Intel binaries with `make build-agent-macos`. For an always-logged-in workstation, copy the matching binary plus `deploy/install-macos-user.sh` and `deploy/com.servterm.agent.user.plist.template`, then run:

```sh
./install-macos-user.sh --binary ./servterm-agent --listen TAILSCALE_IP:7843 --node office-mac
```

The installer creates a `com.servterm.agent` LaunchAgent, a private bearer token, and the SQLite database under `~/Library/Application Support/servterm`. It binds directly to the supplied Tailscale address. A root system-wide alternative is provided by `deploy/install-macos.sh` and `deploy/com.servterm.agent.plist.template`.

Bind `SERVTERM_LISTEN` to the server's specific Tailscale IPv4 address. The agent refuses a non-loopback listener without `SERVTERM_AGENT_TOKEN`. Do not bind it to `0.0.0.0`; the API is plain HTTP because Tailscale already supplies peer authentication and transport encryption, while the independent bearer token prevents other tailnet nodes from reading metrics.

The agent provides:

- `GET /v1/status`: non-sensitive health and protocol version
- `GET /v1/history`: authenticated SQLite-backed history
- `GET /v1/stream`: authenticated WebSocket samples

The optional `servterm-runner-probe` service runs locally as root with no network listener. It reads runner process namespaces, allowlists only runner/repository/workflow/job/run identity fields, and writes sanitized per-job CPU tick/RSS records to `/run/servterm/runner-jobs.jsonl`. The network-facing agent remains unprivileged and cannot read arbitrary runner environments or the Docker socket.

Accelerator activity uses the same least-privilege split. On Linux, install `linux-perf` and run `deploy/servterm-accelerator-probe.service`; it samples the i915 PMU's GPU-awake residency and writes only a device label and percentage to `/run/servterm/accelerators.tsv`. Intel GNA is identified when present, but remains explicitly unavailable because its PCI function exposes no utilization counter on supported kernels. `intel_gpu_top` may be used independently when the kernel publishes engine counters, but is not required by servterm.

On Apple Silicon, GPU and Neural Engine active residency come from Apple's root-only `powermetrics` sampler. A user LaunchAgent must not be granted that privilege. Build the macOS probe, copy its binary and the two deployment files to the Mac, then install the narrow LaunchDaemon once from an administrator shell:

```sh
sudo ./install-macos-accelerator-probe.sh ./servterm-accelerator-probe-darwin-arm64 USER
```

The LaunchDaemon has no network listener and writes only sanitized activity records under that user's private servterm state directory. The network-facing agent continues to run as the ordinary user.

History uses a sliding window: one-second samples for the first hour, one-minute points through 24 hours, five-minute points through seven days, and thirty-minute points through 30 days. Live rendering uses a one-second jitter buffer and calm 10 FPS interpolation.

Cloudflare is not required in the direct path. A future Worker/D1 control plane can provide device enrollment, discovery, alert routing, and offline summaries; live samples should stay peer-to-peer when Tailscale connectivity is available.

The latency shown is the complete SSH collection round trip, not ICMP ping. It measures the actual path used to obtain monitoring data.

## Keys

| Key | Action |
| --- | --- |
| `up` / `down`, `j` / `k` | Select a server |
| `enter`, `right`, `l` | Open details |
| `esc`, `left`, `h` | Return to overview |
| `r` | Refresh now |
| `q`, `ctrl-c` | Quit |

## Security model

- Fallback SSH collection is read-only and uses `BatchMode=yes`, so it never waits on a password prompt.
- Keys are not parsed by `servterm`; an optional identity path is passed to system `ssh`.
- The example contains placeholders only, and common local inventory names are gitignored.
- SSH failures are isolated and shown per host.

## Development

```sh
make check
make build
make build-agent-linux
```

The code separates config validation, Linux collection/parsing, derived rates, and terminal UI so a future daemon or JSON output mode can reuse the collector.
