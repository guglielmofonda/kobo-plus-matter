#!/bin/sh
set -eu

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
  echo "No mounted Kobo volume found."
  exit 1
fi

if [ ! -d "$VOLUME/.kobo" ]; then
  echo "Not a Kobo volume, or .kobo is missing: $VOLUME"
  exit 1
fi

echo "Kobo volume: $VOLUME"

if [ -f "$VOLUME/.kobo/version" ]; then
  version="$(cat "$VOLUME/.kobo/version")"
  echo "Version: $version"
  firmware="$(printf '%s' "$version" | awk -F, '{print $3}')"
  if [ -n "$firmware" ]; then
    case "$firmware" in
      5.*)
        echo "WARNING: firmware $firmware is 5.x; NickelMenu currently documents no 5.x support."
        ;;
      4.*)
        echo "Firmware compatibility: 4.x"
        ;;
      *)
        echo "Firmware compatibility: unknown"
        ;;
    esac
  fi
else
  echo "Version: missing"
fi

if [ -f "$VOLUME/.kobo/KoboRoot.tgz" ]; then
  echo "Updater: pending KoboRoot.tgz still present"
else
  echo "Updater: no pending KoboRoot.tgz"
fi

missing=0
check_path() {
  path="$1"
  if [ -e "$VOLUME/$path" ]; then
    echo "OK: $path"
  else
    echo "MISSING: $path"
    missing=1
  fi
}

check_path ".adds/matter/bin/matter-kobo-sync"
check_path ".adds/matter/cacert.pem"
check_path ".adds/matter/config.env.sample"
check_path ".adds/matter/run-daemon.sh"
check_path ".adds/matter/run-sync.sh"
check_path ".adds/matter/status.sh"
check_path ".adds/matter/stop-daemon.sh"
check_path ".adds/nm/matter"
check_path ".adds/nm/doc"
check_path ".adds/nickeldbus"
check_path "Matter/Matter Setup.txt"

if [ -f "$VOLUME/.adds/matter/config.env" ]; then
  if grep -q '^MATTER_TOKEN=mat_' "$VOLUME/.adds/matter/config.env"; then
    echo "Matter token: configured"
  else
    echo "Matter token: missing or placeholder"
  fi
else
  echo "Matter token: config.env missing"
fi

if [ -f "$VOLUME/.adds/matter/cacert.pem" ]; then
  ca_size="$(wc -c < "$VOLUME/.adds/matter/cacert.pem" | tr -d ' ')"
  if [ "$ca_size" -lt 1024 ]; then
    echo "Matter CA bundle: unexpectedly small"
    missing=1
  else
    echo "Matter CA bundle: present"
  fi
fi

if [ "$missing" -ne 0 ]; then
  echo "Mounted Kobo verification failed."
  exit 1
fi

echo "Mounted Kobo verification passed for visible onboard files."
