#!/bin/sh
set -eu
if [ "$(id -u)" -ne 0 ]; then echo "run as root" >&2; exit 1; fi
if [ "$#" -ne 2 ]; then echo "usage: $0 BINARY USER" >&2; exit 2; fi
binary=$1
user_name=$2
user_home=$(dscl . -read "/Users/$user_name" NFSHomeDirectory | awk '{print $2}')
state_dir="$user_home/Library/Application Support/servterm"
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
plist=/Library/LaunchDaemons/com.servterm.accelerator-probe.plist
install -o root -g wheel -m 0755 "$binary" /usr/local/bin/servterm-accelerator-probe
mkdir -p "$state_dir"
chown "$user_name":staff "$state_dir"
chmod 0700 "$state_dir"
escaped_state=$(printf '%s' "$state_dir" | sed 's/[&|]/\\&/g')
tmp=$(mktemp /tmp/com.servterm.accelerator-probe.plist.XXXXXX)
trap 'rm -f "$tmp"' EXIT
sed "s|__STATE_DIR__|$escaped_state|g" "$script_dir/com.servterm.accelerator-probe.plist.template" > "$tmp"
plutil -lint "$tmp" >/dev/null
install -o root -g wheel -m 0644 "$tmp" "$plist"
launchctl bootout system/com.servterm.accelerator-probe 2>/dev/null || true
launchctl bootstrap system "$plist"
launchctl enable system/com.servterm.accelerator-probe
launchctl kickstart -k system/com.servterm.accelerator-probe
echo "servterm accelerator probe installed for $user_name"
