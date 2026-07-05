#!/bin/sh
set -eu

BASE=/mnt/onboard/.adds/matter
BIN="$BASE/bin/matter-kobo-sync"
CONFIG="$BASE/config.env"
SAMPLE="$BASE/config.env.sample"
PIDFILE="$BASE/matter-daemon.pid"

if [ ! -f "$CONFIG" ] && [ -f "$SAMPLE" ]; then
  cp "$SAMPLE" "$CONFIG"
fi

if [ ! -x "$BIN" ]; then
  exit 0
fi

if [ -f "$CONFIG" ] && grep -q '^AUTO_SYNC_ENABLED=0' "$CONFIG"; then
  exit 0
fi

if [ -f "$PIDFILE" ]; then
  oldpid="$(cat "$PIDFILE" 2>/dev/null || true)"
  if [ -n "$oldpid" ] && kill -0 "$oldpid" 2>/dev/null; then
    exit 0
  fi
fi

"$BIN" daemon --config "$CONFIG" &
echo "$!" > "$PIDFILE"
