#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
KOBOROOT="$ROOT/dist/KoboRoot.tgz"
BINARY="$ROOT/build/matter-kobo-sync"

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

if [ ! -f "$KOBOROOT" ]; then
  echo "Missing $KOBOROOT. Build the package first."
  exit 1
fi

tmp="${TMPDIR:-/tmp}/matter-kobo-update.$$"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT INT TERM

mkdir -p "$tmp"
tar -xzf "$KOBOROOT" -C "$tmp"

copy_file() {
  src="$1"
  dst="$2"
  mode="$3"
  if [ ! -f "$src" ]; then
    echo "Missing staged file: $src"
    exit 1
  fi
  mkdir -p "$(dirname -- "$dst")"
  install -m "$mode" "$src" "$dst"
}

copy_file "$tmp/mnt/onboard/.adds/matter/cacert.pem" "$VOLUME/.adds/matter/cacert.pem" 644
copy_file "$tmp/mnt/onboard/.adds/matter/config.env.sample" "$VOLUME/.adds/matter/config.env.sample" 644
copy_file "$tmp/mnt/onboard/.adds/matter/run-daemon.sh" "$VOLUME/.adds/matter/run-daemon.sh" 755
copy_file "$tmp/mnt/onboard/.adds/matter/run-sync.sh" "$VOLUME/.adds/matter/run-sync.sh" 755
copy_file "$tmp/mnt/onboard/.adds/matter/status.sh" "$VOLUME/.adds/matter/status.sh" 755
copy_file "$tmp/mnt/onboard/.adds/matter/stop-daemon.sh" "$VOLUME/.adds/matter/stop-daemon.sh" 755
copy_file "$tmp/mnt/onboard/.adds/nm/matter" "$VOLUME/.adds/nm/matter" 644
copy_file "$tmp/mnt/onboard/.adds/nm/doc" "$VOLUME/.adds/nm/doc" 644
copy_file "$tmp/mnt/onboard/.adds/nickeldbus" "$VOLUME/.adds/nickeldbus" 644
copy_file "$tmp/mnt/onboard/Matter/Matter Setup.txt" "$VOLUME/Matter/Matter Setup.txt" 644

if [ -f "$BINARY" ]; then
  copy_file "$BINARY" "$VOLUME/.adds/matter/bin/matter-kobo-sync" 755
else
  copy_file "$tmp/mnt/onboard/.adds/matter/bin/matter-kobo-sync" "$VOLUME/.adds/matter/bin/matter-kobo-sync" 755
fi

sync

echo "Updated visible Matter files on $VOLUME"
if [ -f "$VOLUME/.adds/matter/config.env" ]; then
  echo "Preserved existing Matter config.env"
else
  echo "Matter config.env is missing; seed the token with dist/seed-matter-token.sh"
fi
