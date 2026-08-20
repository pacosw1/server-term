#!/bin/sh
set -eu
usage() { echo "usage: sudo $0 --binary PATH --listen IP:PORT --node NAME --platform linux --token TOKEN_FILE --vnc-password PASSWORD_FILE [--backend NAME]" >&2; exit 2; }
binary= listen= node= platform= token= password= backend=auto
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
while [ "$#" -gt 0 ]; do
  case "$1" in
    --binary) [ "$#" -ge 2 ] || usage; binary=$2; shift 2;;
    --listen) [ "$#" -ge 2 ] || usage; listen=$2; shift 2;;
    --node) [ "$#" -ge 2 ] || usage; node=$2; shift 2;;
    --platform) [ "$#" -ge 2 ] || usage; platform=$2; shift 2;;
    --token) [ "$#" -ge 2 ] || usage; token=$2; shift 2;;
    --vnc-password) [ "$#" -ge 2 ] || usage; password=$2; shift 2;;
    --backend) [ "$#" -ge 2 ] || usage; backend=$2; shift 2;;
    *) usage;;
  esac
done
[ "$(id -u)" -eq 0 ] && [ -f "$binary" ] && [ -n "$listen" ] && [ -n "$node" ] && [ -n "$platform" ] && [ -f "$token" ] && [ -f "$password" ] || usage
if [ "$backend" != auto ] && ! command -v "$backend" >/dev/null 2>&1; then echo "desktop backend '$backend' is not installed" >&2; exit 1; fi
if ! id servterm-desktop >/dev/null 2>&1; then useradd --system --home-dir /nonexistent --shell /usr/sbin/nologin servterm-desktop; fi
install -o root -g root -m 0755 "$binary" /usr/local/bin/servterm-desktop-agent
install -d -o root -g servterm-desktop -m 0750 /etc/servterm
install -o root -g servterm-desktop -m 0640 "$token" /etc/servterm/desktop-agent.token
install -o root -g servterm-desktop -m 0640 "$password" /etc/servterm/desktop-vnc.password
cat > /etc/servterm/desktop-agent.env <<EOF
SERVTERM_LISTEN=$listen
SERVTERM_NODE=$node
SERVTERM_PLATFORM=$platform
SERVTERM_BACKEND=$backend
SERVTERM_VNC_HOST=127.0.0.1
SERVTERM_VNC_PORT=5900
EOF
chown root:servterm-desktop /etc/servterm/desktop-agent.env
chmod 0640 /etc/servterm/desktop-agent.env
install -m 0644 "$script_dir/servterm-desktop-agent.service" /etc/systemd/system/servterm-desktop-agent.service
systemctl daemon-reload
systemctl enable --now servterm-desktop-agent
systemctl is-active --quiet servterm-desktop-agent
echo "servterm desktop agent installed; backend capability is reported by /v1/status"
