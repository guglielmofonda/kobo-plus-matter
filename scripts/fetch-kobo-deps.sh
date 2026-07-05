#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
VENDOR="$ROOT/.vendor"
mkdir -p "$VENDOR"

curl -L -f \
  -o "$VENDOR/NickelMenu-v0.6.0-KoboRoot.tgz" \
  "https://github.com/pgaskin/NickelMenu/releases/download/v0.6.0/KoboRoot.tgz"

curl -L -f \
  -o "$VENDOR/NickelDBus-0.2.0-KoboRoot.tgz" \
  "https://github.com/shermp/NickelDBus/releases/download/0.2.0/KoboRoot.tgz"

shasum -a 256 \
  "$VENDOR/NickelMenu-v0.6.0-KoboRoot.tgz" \
  "$VENDOR/NickelDBus-0.2.0-KoboRoot.tgz"

