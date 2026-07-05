#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
RESTORE="$ROOT/dist/KoboRoot.home-cleanup-restore.tgz"
TARGET_FIRMWARE="4.38.23697"

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
  echo "Refusing to install home-cleanup restore on firmware $firmware."
  echo "This restore package is only for firmware $TARGET_FIRMWARE."
  exit 1
fi

if ! tar -tzf "$RESTORE" | sed 's|^\./||' | grep -qx 'usr/local/Kobo/libnickel.so.1.0.0'; then
  echo "Restore archive does not contain usr/local/Kobo/libnickel.so.1.0.0"
  exit 1
fi

if ! tar -tzf "$RESTORE" | sed 's|^\./||' | grep -qx 'usr/local/Kobo/nickel'; then
  echo "Restore archive does not contain usr/local/Kobo/nickel"
  exit 1
fi

pending="$VOLUME/.kobo/KoboRoot.tgz"
if [ -f "$pending" ]; then
  backup="$VOLUME/.kobo/KoboRoot.before-home-cleanup-restore.$(date -u '+%Y%m%dT%H%M%SZ').tgz"
  cp -p "$pending" "$backup"
  echo "Backed up existing pending Kobo update to $backup"
fi

cp "$RESTORE" "$pending"
sync

echo "Copied home-cleanup restore package to $pending"
echo "Eject the Kobo and let it reboot."
