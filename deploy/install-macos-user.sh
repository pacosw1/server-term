#!/bin/sh
set -eu

usage() {
  echo "usage: $0 --binary PATH --listen TAILSCALE_IP:PORT [--node NAME]" >&2
  exit 2
}

binary= listen= node=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --binary) [ "$#" -ge 2 ] || usage; binary=$2; shift 2 ;;
    --listen) [ "$#" -ge 2 ] || usage; listen=$2; shift 2 ;;
    --node) [ "$#" -ge 2 ] || usage; node=$2; shift 2 ;;
    *) usage ;;
  esac
done
[ "$(id -u)" -ne 0 ] || { echo "use install-macos.sh for a root system install" >&2; exit 1; }
[ -f "$binary" ] && [ -n "$listen" ] || usage
[ -n "$node" ] || node=$(scutil --get LocalHostName 2>/dev/null || hostname -s)
user_home=$HOME
state_dir="$user_home/Library/Application Support/servterm"
plist_dir="$user_home/Library/LaunchAgents"
plist="$plist_dir/com.servterm.agent.plist"

install -d -m 0700 "$state_dir"
install -d -m 0755 "$user_home/.local/bin" "$plist_dir"
if [ ! -s "$state_dir/agent.token" ]; then
  umask 077
  openssl rand -hex 32 > "$state_dir/agent.token"
fi
chmod 0600 "$state_dir/agent.token"
install -m 0755 "$binary" "$user_home/.local/bin/servterm-agent"

template_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
escape() { printf '%s' "$1" | sed 's/[&|]/\\&/g'; }
plist_tmp=$(mktemp /tmp/com.servterm.agent.user.plist.XXXXXX)
trap 'rm -f "$plist_tmp"' EXIT HUP INT TERM
sed -e "s|__HOME__|$(escape "$user_home")|g" \
    -e "s|__STATE_DIR__|$(escape "$state_dir")|g" \
    -e "s|__LISTEN__|$(escape "$listen")|g" \
    -e "s|__NODE__|$(escape "$node")|g" \
    "$template_dir/com.servterm.agent.user.plist.template" > "$plist_tmp"
plutil -lint "$plist_tmp" >/dev/null
install -m 0644 "$plist_tmp" "$plist"
domain="gui/$(id -u)"
launchctl bootout "$domain/com.servterm.agent" 2>/dev/null || true
launchctl bootstrap "$domain" "$plist"
launchctl enable "$domain/com.servterm.agent"
launchctl kickstart -k "$domain/com.servterm.agent"
echo "servterm agent installed at $listen (node $node)"
