#!/bin/sh
set -eu

BASE=/mnt/onboard/.adds/matter
BIN="$BASE/bin/matter-kobo-sync"
CONFIG="$BASE/config.env"
SAMPLE="$BASE/config.env.sample"

if [ ! -f "$CONFIG" ] && [ -f "$SAMPLE" ]; then
  cp "$SAMPLE" "$CONFIG"
fi

if [ ! -x "$BIN" ]; then
  echo "Matter sync binary is missing."
  exit 1
fi

exec "$BIN" status --config "$CONFIG"
