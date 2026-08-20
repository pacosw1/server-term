#!/bin/sh
set -eu
usage() { echo "usage: $0 --binary PATH --listen IP:PORT --node NAME --token TOKEN_FILE [--vnc-password PASSWORD_FILE]" >&2; exit 2; }
binary= listen= node= token= password=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --binary) [ "$#" -ge 2 ] || usage; binary=$2; shift 2;;
    --listen) [ "$#" -ge 2 ] || usage; listen=$2; shift 2;;
    --node) [ "$#" -ge 2 ] || usage; node=$2; shift 2;;
    --token) [ "$#" -ge 2 ] || usage; token=$2; shift 2;;
    --vnc-password) [ "$#" -ge 2 ] || usage; password=$2; shift 2;;
    *) usage;;
  esac
done
[ "$(id -u)" -ne 0 ] && [ -f "$binary" ] && [ -f "$token" ] && [ -n "$listen" ] && [ -n "$node" ] || usage
state="$HOME/Library/Application Support/servterm"
plist_dir="$HOME/Library/LaunchAgents"
plist="$plist_dir/com.servterm.desktop-agent.plist"
install -d -m 0700 "$state" "$plist_dir" "$HOME/.local/bin"
install -m 0755 "$binary" "$HOME/.local/bin/servterm-desktop-agent"
install -m 0600 "$token" "$state/desktop-agent.token"
template_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
escape() { printf '%s' "$1" | sed 's/[&|]/\\&/g'; }
tmp=$(mktemp /tmp/com.servterm.desktop-agent.plist.XXXXXX)
trap 'rm -f "$tmp"' EXIT HUP INT TERM
sed -e "s|__HOME__|$(escape "$HOME")|g" -e "s|__STATE__|$(escape "$state")|g" -e "s|__LISTEN__|$(escape "$listen")|g" -e "s|__NODE__|$(escape "$node")|g" "$template_dir/com.servterm.desktop-agent.plist.template" > "$tmp"
plutil -lint "$tmp" >/dev/null
install -m 0644 "$tmp" "$plist"
domain="gui/$(id -u)"
launchctl bootout "$domain/com.servterm.desktop-agent" 2>/dev/null || true
launchctl bootstrap "$domain" "$plist"
launchctl enable "$domain/com.servterm.desktop-agent"
launchctl kickstart -k "$domain/com.servterm.desktop-agent"
echo "servterm desktop agent installed at $listen (node $node)"
