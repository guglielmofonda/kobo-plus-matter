#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-900}"
SLEEP_SECONDS="${SLEEP_SECONDS:-5}"
deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))

echo "Waiting up to $TIMEOUT_SECONDS seconds for a mounted Kobo..."

while [ "$(date +%s)" -le "$deadline" ]; do
  for candidate in /Volumes/*; do
    if [ -d "$candidate/.kobo" ]; then
      echo "Found Kobo at $candidate"
      exec sh "$ROOT/dist/verify-mounted-kobo.sh" "$candidate"
    fi
  done
  sleep "$SLEEP_SECONDS"
done

echo "Timed out waiting for Kobo."
exit 1

