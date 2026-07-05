#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
GO_BIN="${GO_BIN:-go}"

mkdir -p "$ROOT/build"
mkdir -p "$ROOT/.cache/go-build"
rm -f "$ROOT/dist/KoboRoot.combined.tgz"

GOCACHE="$ROOT/.cache/go-build" GOOS=linux GOARCH=arm GOARM=6 CGO_ENABLED=0 "$GO_BIN" build \
  -trimpath \
  -ldflags "-s -w" \
  -o "$ROOT/build/matter-kobo-sync" \
  "$ROOT/cmd/matter-kobo-sync"

set -- "$ROOT/scripts/package.py" --binary "$ROOT/build/matter-kobo-sync"
if [ -f "$ROOT/.vendor/NickelMenu-v0.6.0-KoboRoot.tgz" ]; then
  set -- "$@" --merge-koboroot "$ROOT/.vendor/NickelMenu-v0.6.0-KoboRoot.tgz"
fi
if [ -f "$ROOT/.vendor/NickelDBus-0.2.0-KoboRoot.tgz" ]; then
  set -- "$@" --merge-koboroot "$ROOT/.vendor/NickelDBus-0.2.0-KoboRoot.tgz"
fi

python3 "$@"
python3 "$ROOT/scripts/verify-package.py"
python3 "$ROOT/scripts/test-install-tools.py"
