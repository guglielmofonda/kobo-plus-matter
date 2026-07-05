#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
OUT="$ROOT/packaging/root/mnt/onboard/.adds/matter/cacert.pem"
TMP="$OUT.tmp"

mkdir -p "$(dirname -- "$OUT")"
rm -f "$TMP"

if command -v security >/dev/null 2>&1 && [ -f /System/Library/Keychains/SystemRootCertificates.keychain ]; then
  security find-certificate -a -p /System/Library/Keychains/SystemRootCertificates.keychain > "$TMP"
fi

if [ ! -s "$TMP" ]; then
  if [ -s /etc/ssl/cert.pem ]; then
    cp /etc/ssl/cert.pem "$TMP"
  else
    echo "refresh-ca-bundle: generated CA bundle is empty" >&2
    exit 1
  fi
fi

mv "$TMP" "$OUT"
echo "wrote $OUT"
