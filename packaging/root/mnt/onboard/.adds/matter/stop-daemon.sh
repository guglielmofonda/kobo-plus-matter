#!/bin/sh
set -eu

BASE=/mnt/onboard/.adds/matter
PIDFILE="$BASE/matter-daemon.pid"

if [ ! -f "$PIDFILE" ]; then
  echo "Matter auto sync is not running."
  exit 0
fi

pid="$(cat "$PIDFILE" 2>/dev/null || true)"
if [ -z "$pid" ]; then
  rm -f "$PIDFILE"
  echo "Matter auto sync is not running."
  exit 0
fi

if kill "$pid" 2>/dev/null; then
  rm -f "$PIDFILE"
  echo "Stopped Matter auto sync."
else
  rm -f "$PIDFILE"
  echo "Matter auto sync was not running."
fi

