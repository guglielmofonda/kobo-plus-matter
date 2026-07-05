#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
FIRMWARE="$ROOT/build/firmware/4.38.23697/kobo-update-4.38.23697.zip"
OFFICIAL_ROOT="$ROOT/build/firmware/4.38.23697/extract"
OFFICIAL_LIB="$OFFICIAL_ROOT/usr/local/Kobo/libnickel.so.1.0.0"
OFFICIAL_NICKEL="$OFFICIAL_ROOT/usr/local/Kobo/nickel"
KOBOPATCH="$ROOT/.tools/kobopatch/kobopatch"
OUT_DIR="$ROOT/build/home-cleanup"
DIST_OUT="$ROOT/dist/KoboRoot.home-cleanup.tgz"
RESTORE_STAGE="$ROOT/build/home-cleanup-restore-root"
RESTORE_OUT="$ROOT/dist/KoboRoot.home-cleanup-restore.tgz"

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

if [ ! -f "$OFFICIAL_LIB" ]; then
  echo "Missing extracted official libnickel: $OFFICIAL_LIB"
  echo "Extract the official firmware KoboRoot.tgz under $OFFICIAL_ROOT first."
  exit 1
fi

if [ ! -f "$OFFICIAL_NICKEL" ]; then
  echo "Missing extracted official nickel executable: $OFFICIAL_NICKEL"
  echo "Extract the official firmware KoboRoot.tgz under $OFFICIAL_ROOT first."
  exit 1
fi

rm -rf "$OUT_DIR" "$RESTORE_STAGE"
mkdir -p "$OUT_DIR" "$ROOT/dist" "$RESTORE_STAGE/usr/local/Kobo"

"$KOBOPATCH" "$ROOT/patches/home-cleanup/kobopatch.yaml"

cp "$OUT_DIR/KoboRoot.home-cleanup.tgz" "$DIST_OUT"
cp "$OFFICIAL_LIB" "$RESTORE_STAGE/usr/local/Kobo/libnickel.so.1.0.0"
cp "$OFFICIAL_NICKEL" "$RESTORE_STAGE/usr/local/Kobo/nickel"

(
  cd "$RESTORE_STAGE"
  tar -czf "$RESTORE_OUT" usr/local/Kobo/libnickel.so.1.0.0 usr/local/Kobo/nickel
)

python3 "$ROOT/scripts/verify-home-cleanup-patch.py" "$DIST_OUT" "$RESTORE_OUT" "$OUT_DIR/kobopatch.log"

echo "wrote $DIST_OUT"
echo "wrote $RESTORE_OUT"
