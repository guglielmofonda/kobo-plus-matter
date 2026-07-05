#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
PATCH="$ROOT/dist/KoboRoot.home-cleanup.tgz"
RESTORE="$ROOT/dist/KoboRoot.home-cleanup-restore.tgz"
COMBINED="$ROOT/dist/KoboRoot.home-cleanup.combined.tgz"
TARGET_FIRMWARE="4.38.23697"

if [ ! -f "$PATCH" ]; then
  echo "Missing $PATCH. Run scripts/build-home-cleanup-patch.sh first."
  exit 1
fi

if [ ! -f "$RESTORE" ]; then
  echo "Missing $RESTORE. Run scripts/build-home-cleanup-patch.sh first."
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

version_file="$VOLUME/.kobo/version"
if [ ! -f "$version_file" ]; then
  echo "Kobo version file not found: $version_file"
  exit 1
fi

firmware="$(awk -F, '{print $3}' "$version_file")"
if [ "$firmware" != "$TARGET_FIRMWARE" ]; then
  echo "Refusing to install home-cleanup patch on firmware $firmware."
  echo "This patch is only verified for firmware $TARGET_FIRMWARE."
  exit 1
fi

if ! tar -tzf "$PATCH" | sed 's|^\./||' | grep -qx 'usr/local/Kobo/libnickel.so.1.0.0'; then
  echo "Patch archive does not contain usr/local/Kobo/libnickel.so.1.0.0"
  exit 1
fi

if ! tar -tzf "$PATCH" | sed 's|^\./||' | grep -qx 'usr/local/Kobo/nickel'; then
  echo "Patch archive does not contain usr/local/Kobo/nickel"
  exit 1
fi

installer="$PATCH"
pending="$VOLUME/.kobo/KoboRoot.tgz"

if [ -f "$pending" ]; then
  pending_hash="$(shasum -a 256 "$pending" | awk '{print $1}')"
  patch_hash="$(shasum -a 256 "$PATCH" | awk '{print $1}')"
  if [ "$pending_hash" != "$patch_hash" ]; then
    backup="$VOLUME/.kobo/KoboRoot.before-home-cleanup.$(date -u '+%Y%m%dT%H%M%SZ').tgz"
    cp -p "$pending" "$backup"
    echo "Backed up existing pending Kobo update to $backup"

    if tar -tzf "$pending" | sed 's|^\./||' | grep -Eqx 'usr/local/Kobo/(libnickel\.so\.1\.0\.0|nickel)'; then
      echo "Refusing to merge over a pending update that already contains Nickel firmware files."
      echo "Let that update install first, then reconnect the Kobo and rerun this helper."
      exit 1
    fi

    python3 "$ROOT/scripts/combine-koboroot.py" \
      --base "$pending" \
      --overlay "$PATCH" \
      --output "$COMBINED"
    installer="$COMBINED"
    echo "Preserving existing pending Kobo update by staging a combined installer."
  fi
fi

cp "$installer" "$pending"
mkdir -p "$VOLUME/.adds/matter"
cp "$RESTORE" "$VOLUME/.adds/matter/KoboRoot.home-cleanup-restore.tgz"
sync

echo "Copied home-cleanup patch to $pending"
echo "Copied restore package to $VOLUME/.adds/matter/KoboRoot.home-cleanup-restore.tgz"
echo "Eject the Kobo and let it reboot."
