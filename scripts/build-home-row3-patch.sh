#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
FIRMWARE="$ROOT/build/firmware/4.38.23697/kobo-update-4.38.23697.zip"
KOBOPATCH="$ROOT/.tools/kobopatch/kobopatch"
OUT_DIR="$ROOT/build/home-row3"
DIST_OUT="$ROOT/dist/KoboRoot.home-row3.tgz"

if [ ! -x "$KOBOPATCH" ]; then
  echo "Missing kobopatch: $KOBOPATCH"
  echo "Download kobopatch v0.16.0 darwin-arm64 to that path first."
  exit 1
fi

if [ ! -f "$FIRMWARE" ]; then
  echo "Missing exact firmware: $FIRMWARE"
  echo "Download https://ereaderfiles.kobo.com/firmwares/kobo6/May2026/kobo-update-4.38.23697.zip"
  exit 1
fi

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR" "$ROOT/dist"

"$KOBOPATCH" "$ROOT/patches/home-row3/kobopatch.yaml"

cp "$OUT_DIR/KoboRoot.home-row3.tgz" "$DIST_OUT"
python3 "$ROOT/scripts/verify-home-row3-patch.py" "$DIST_OUT" "$OUT_DIR/kobopatch.log"

echo "wrote $DIST_OUT"
