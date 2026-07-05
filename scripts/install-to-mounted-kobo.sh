#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
KOBOROOT="$ROOT/dist/KoboRoot.tgz"
COMBINED="$ROOT/dist/KoboRoot.combined.tgz"

if [ ! -f "$KOBOROOT" ]; then
  echo "Missing $KOBOROOT. Build the package first."
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
  echo "No mounted Kobo volume found. Connect the Kobo over USB first."
  exit 1
fi

if [ ! -d "$VOLUME/.kobo" ]; then
  echo "Not a Kobo volume, or .kobo is missing: $VOLUME"
  exit 1
fi

INSTALLER="$KOBOROOT"
if [ -f "$VOLUME/.kobo/KoboRoot.tgz" ]; then
  mounted_hash="$(shasum -a 256 "$VOLUME/.kobo/KoboRoot.tgz" | awk '{print $1}')"
  local_hash="$(shasum -a 256 "$KOBOROOT" | awk '{print $1}')"
  if [ "$mounted_hash" != "$local_hash" ]; then
    BACKUP="$VOLUME/.kobo/KoboRoot.before-matter.tgz"
    if [ ! -f "$BACKUP" ]; then
      cp -p "$VOLUME/.kobo/KoboRoot.tgz" "$BACKUP"
      echo "Backed up existing Kobo update to $BACKUP"
    fi
    python3 "$ROOT/scripts/combine-koboroot.py" \
      --base "$VOLUME/.kobo/KoboRoot.tgz" \
      --overlay "$KOBOROOT" \
      --output "$COMBINED"
    INSTALLER="$COMBINED"
    echo "Preserving existing Kobo update by staging a combined installer."
  fi
fi

cp "$INSTALLER" "$VOLUME/.kobo/KoboRoot.tgz"

if [ -n "${MATTER_TOKEN:-}" ]; then
  mkdir -p "$VOLUME/.adds/matter"
  sed "s|^MATTER_TOKEN=.*|MATTER_TOKEN=$MATTER_TOKEN|" \
    "$ROOT/packaging/root/mnt/onboard/.adds/matter/config.env.sample" \
    > "$VOLUME/.adds/matter/config.env"
  echo "Wrote Matter token to $VOLUME/.adds/matter/config.env"
fi

sync

echo "Copied installer to $VOLUME/.kobo/KoboRoot.tgz"
echo "Eject the Kobo and let it reboot."
