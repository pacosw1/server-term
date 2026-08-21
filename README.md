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
servterm ssh hetzner-32cpu
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

### MCP for Codex/Claude

Run Servterm as a local stdio MCP server. The MCP process reads the protected
inventory and token files locally; models receive only sanitized read-only
results, never SSH keys or bearer tokens:

```sh
servterm --config "$HOME/Library/Application Support/servterm/config.yaml" mcp
```

Register that command with the MCP configuration used by Codex or Claude. It
provides `servterm_list_servers`, `servterm_status`, `servterm_history`,
`servterm_stream`, `servterm_list_desktops`, `servterm_desktop_status`,
`servterm_nvr_status`, and `servterm_orchestrator_status`. Stream calls are
bounded to ten samples and there are no arbitrary shell, SSH, desktop-input,
or credential-reading tools.

`servterm ssh NAME` hands the terminal directly to the configured OpenSSH
connection (and accepts extra SSH arguments). It is the low-latency terminal-
only path; desktop capture/control is intentionally a separate surface.

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

### Agent orchestrator widget provider

Add an agent orchestrator status provider to the inventory:

```yaml
widgets:
  - name: pitsa-agents
    type: orchestrator
    endpoint: http://100.93.34.43:7844
    token_file: ~/Library/Application Support/servterm/tokens/pitsa-agents
```

The AGENTS tab appears only in the detail view of the server that runs the
daemon. servterm finds that server by the endpoint host. Add `host: <server
address>` to the widget when the endpoint points somewhere else, for example
through a local port forward.

Query its normalized snapshot with `servterm widget --host pitsa-agents --json`.
Reading is entirely `GET /api/status`; it never starts, stops, or steers an
agent. It reports which account is paying (a subscription plan, an API key,
or unknown), the daemon mode, the weekly and five-hour subscription plan
usage, the daemon host's overall disk usage, and the live agent list (issue,
state, cycle, branch, worktree path and its disk usage, PR, weekly plan
share, turns, last activity, tokens, cost, PID, CPU/RSS, and the last error)
including any spark subagents a task launched and any self-tracked task
checklist.

The widget has exactly one write: `POST /api/mode`, to switch the daemon
between `fast` (full fanout), `economy` (one third), and `paused` (takes no
new work; agents already running finish their task — it is not a kill
switch). Every mode only reduces work; none can raise fanout or spend, turn
on autoMerge, or change the repository, so a mistaken or hostile call can do
no worse than quiet the daemon down. Nothing else on the daemon is remotely
writable — budgets, fanout limits, `autoMerge`, the repository, and the
model stay in the config file on the machine, changed only by an operator
with ssh.

**The dollar figure is not always real money.** On a subscription account a
per-call price does not exist, so the figure is a computed estimate
(`tokens × a constant`), marked `est ~$X/$Y day`. An API key account's
figure is real billed spend, shown plainly with no `~`. An account the
daemon could not identify is still treated as billed, shown as
`$X/$Y day billed` — it is never marked as an estimate, since that would
hide real spending. `servterm widget` prints which account is in use right
next to the figure (`codex pro`, `api key`, or `unknown account`).

In the interactive TUI, open a server's detail view and press `o` (or `tab`
to cycle) to reach the AGENTS tab. Use `up`/`down` (or `j`/`k`) to select an
agent; the selected agent's full detail — including its worktree path and
disk usage, a live CPU/memory trend, its spark subagent tree, and its task
checklist (done items struck through) when present — is shown below the
list. Press `i` to open the selected agent's GitHub issue in the browser, or
`p` to open its pull request (a no-op when it has none yet). Press `m` to
open the mode menu: `up`/`down` (or `j`/`k`) highlights `fast`/`economy`/
`paused`, one `Enter` arms the change, a second `Enter` sends it, and `Esc`
cancels at any point before that second `Enter`. The daemon's own success or
error text is shown, never an invented one, and the header's mode only
updates once the daemon confirms it, not on the local request.

Three fields are still daemon-side work in progress and render nothing until
the daemon sends them, exactly like the fields above: `disk` (host disk
usage), an agent's `worktree`/`worktree_disk_bytes`, and an agent's `tasks`
checklist. `tasks` is `[{"text": "...", "done": false}]`; a hook that keeps
an agent's own checklist current (reminding it to add and check off steps as
it works) is orchestrator/agent-runtime behavior, not something this TUI
repo can provide — the daemon just needs to publish whatever checklist state
it already has under `tasks` for the widget to display it.

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
