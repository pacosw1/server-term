#!/bin/sh
set -eu

usage() {
  echo "usage: sudo $0 --binary PATH --listen TAILSCALE_IP:PORT [--node NAME] [--user USER]" >&2
  exit 2
}

binary= listen= node= user_name=${SUDO_USER:-}
while [ "$#" -gt 0 ]; do
  case "$1" in
    --binary) [ "$#" -ge 2 ] || usage; binary=$2; shift 2 ;;
    --listen) [ "$#" -ge 2 ] || usage; listen=$2; shift 2 ;;
    --node) [ "$#" -ge 2 ] || usage; node=$2; shift 2 ;;
    --user) [ "$#" -ge 2 ] || usage; user_name=$2; shift 2 ;;
    *) usage ;;
  esac
done
[ "$(id -u)" -eq 0 ] || { echo "install-macos.sh must run as root" >&2; exit 1; }
[ -f "$binary" ] && [ -n "$listen" ] && [ -n "$user_name" ] || usage
user_home=$(dscl . -read "/Users/$user_name" NFSHomeDirectory | awk '{print $2}')
[ -n "$node" ] || node=$(scutil --get LocalHostName 2>/dev/null || hostname -s)
state_dir="$user_home/Library/Application Support/servterm"
plist=/Library/LaunchDaemons/com.servterm.agent.plist

install -d -o "$user_name" -g staff -m 0700 "$state_dir"
if [ ! -s "$state_dir/agent.token" ]; then
  umask 077
  openssl rand -hex 32 > "$state_dir/agent.token"
fi
chown "$user_name":staff "$state_dir/agent.token"
chmod 0600 "$state_dir/agent.token"
install -o root -g wheel -m 0755 "$binary" /usr/local/bin/servterm-agent

template_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
escaped_state=$(printf '%s' "$state_dir" | sed 's/[&|]/\\&/g')
escaped_listen=$(printf '%s' "$listen" | sed 's/[&|]/\\&/g')
escaped_node=$(printf '%s' "$node" | sed 's/[&|]/\\&/g')
escaped_user=$(printf '%s' "$user_name" | sed 's/[&|]/\\&/g')
plist_tmp=$(mktemp /tmp/com.servterm.agent.plist.XXXXXX)
trap 'rm -f "$plist_tmp"' EXIT HUP INT TERM
sed -e "s|__STATE_DIR__|$escaped_state|g" -e "s|__LISTEN__|$escaped_listen|g" -e "s|__NODE__|$escaped_node|g" -e "s|__USER__|$escaped_user|g" "$template_dir/com.servterm.agent.plist.template" > "$plist_tmp"
plutil -lint "$plist_tmp" >/dev/null
install -o root -g wheel -m 0644 "$plist_tmp" "$plist"
launchctl bootout system/com.servterm.agent 2>/dev/null || true
launchctl bootstrap system "$plist"
launchctl enable system/com.servterm.agent
launchctl kickstart -k system/com.servterm.agent
echo "servterm agent installed for $user_name at $listen (node $node)"
