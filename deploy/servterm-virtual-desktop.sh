#!/bin/sh
set -eu
display=${SERVTERM_DISPLAY:-:99}
screen=${SERVTERM_SCREEN:-1280x800x24}
Xvfb "$display" -screen 0 "$screen" -nolisten tcp -ac &
xvfb=$!
trap 'kill "$xvfb" 2>/dev/null || true; kill "$wm" "$term" 2>/dev/null || true' EXIT TERM INT
sleep 1
DISPLAY="$display" openbox-session >/run/servterm/openbox.log 2>&1 & wm=$!
DISPLAY="$display" xterm -geometry 120x35+20+20 -title ServtermVirtualDesktop >/run/servterm/xterm.log 2>&1 & term=$!
exec x11vnc -display "$display" -localhost -rfbport 5900 -nopw -forever -shared -noxdamage
