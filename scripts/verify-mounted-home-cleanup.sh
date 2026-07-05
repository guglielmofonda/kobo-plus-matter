#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
PATCH="$ROOT/dist/KoboRoot.home-cleanup.tgz"
RESTORE="$ROOT/dist/KoboRoot.home-cleanup-restore.tgz"
TARGET_FIRMWARE="4.38.23697"

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
  echo "Unexpected firmware $firmware; home-cleanup patch targets $TARGET_FIRMWARE."
  exit 1
fi

pending="$VOLUME/.kobo/KoboRoot.tgz"
if [ -f "$pending" ]; then
  if [ -f "$PATCH" ]; then
    pending_hash="$(shasum -a 256 "$pending" | awk '{print $1}')"
    patch_hash="$(shasum -a 256 "$PATCH" | awk '{print $1}')"
    if [ "$pending_hash" = "$patch_hash" ]; then
      echo "Home-cleanup update is still pending at $pending; eject and let the Kobo reboot."
      exit 2
    fi
  fi
  echo "Unexpected pending Kobo update remains at $pending"
  exit 1
fi

device_restore="$VOLUME/.adds/matter/KoboRoot.home-cleanup-restore.tgz"
if [ ! -f "$device_restore" ]; then
  echo "Missing restore package on Kobo: $device_restore"
  exit 1
fi

if [ -f "$RESTORE" ]; then
  device_restore_hash="$(shasum -a 256 "$device_restore" | awk '{print $1}')"
  restore_hash="$(shasum -a 256 "$RESTORE" | awk '{print $1}')"
  if [ "$device_restore_hash" != "$restore_hash" ]; then
    echo "Restore package hash mismatch on Kobo"
    echo "expected $restore_hash"
    echo "actual   $device_restore_hash"
    exit 1
  fi
fi

echo "Home-cleanup updater was consumed on firmware $firmware."
echo "Matching restore package is present at $device_restore."
