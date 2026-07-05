#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

if [ -z "${MATTER_TOKEN:-}" ]; then
  echo "Set MATTER_TOKEN before running this script."
  echo "Example: MATTER_TOKEN=mat_your_token_here sh dist/seed-matter-token.sh"
  exit 1
fi

if [ "$#" -gt 0 ]; then
  VOLUME="$1"
else
  VOLUME=""
  for candidate in /Volumes/*; do
    if [ -d "$candidate/.kobo" ]; then
      VOLUME="$candidate"
      break
    fi
  done
fi

if [ -z "$VOLUME" ]; then
  echo "No mounted Kobo volume found."
  exit 1
fi

if [ ! -d "$VOLUME/.kobo" ]; then
  echo "Not a Kobo volume, or .kobo is missing: $VOLUME"
  exit 1
fi

mkdir -p "$VOLUME/.adds/matter"

sample="$VOLUME/.adds/matter/config.env.sample"
if [ ! -f "$sample" ]; then
  sample="$ROOT/packaging/root/mnt/onboard/.adds/matter/config.env.sample"
fi

if [ ! -f "$sample" ]; then
  echo "Missing config sample. Rebuild or reconnect after the installer has run."
  exit 1
fi

sed "s|^MATTER_TOKEN=.*|MATTER_TOKEN=$MATTER_TOKEN|" "$sample" > "$VOLUME/.adds/matter/config.env"
sync

echo "Wrote Matter token to $VOLUME/.adds/matter/config.env"

