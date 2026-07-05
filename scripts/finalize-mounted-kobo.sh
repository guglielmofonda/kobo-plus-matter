#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

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

sh "$ROOT/dist/update-mounted-kobo.sh" "$VOLUME"
sh "$ROOT/dist/verify-mounted-kobo.sh" "$VOLUME"

if [ -f "$VOLUME/.kobo/KoboReader.sqlite" ]; then
  sh "$ROOT/dist/label-mounted-kobo-matter.sh" "$VOLUME"
else
  echo "Skipping Matter collection label: KoboReader.sqlite not found."
fi

epub_count="$(find "$VOLUME/Matter" -maxdepth 1 -type f -name '*.epub' 2>/dev/null | wc -l | tr -d ' ')"
echo "Matter EPUB files on disk: $epub_count"

if [ -f "$VOLUME/.adds/matter/state.json" ]; then
  python3 - "$VOLUME/.adds/matter/state.json" <<'PY'
import json
import sys
from pathlib import Path

state_path = Path(sys.argv[1])
try:
    state = json.loads(state_path.read_text())
except Exception as exc:
    print(f"Could not parse Matter state: {exc}")
    raise SystemExit(0)

print(f"Last sync started: {state.get('last_sync_started_at') or 'missing'}")
print(f"Last sync finished: {state.get('last_sync_finished_at') or 'missing'}")
if state.get("last_error"):
    print(f"Last sync error: {state['last_error']}")
print(f"Matter state items: {len(state.get('items') or {})}")
versions = sorted({
    item.get("epub_metadata_version", "")
    for item in (state.get("items") or {}).values()
})
versions = [version for version in versions if version]
if versions:
    print("EPUB metadata versions: " + ", ".join(versions))
PY
fi

if command -v sqlite3 >/dev/null 2>&1 && [ -f "$VOLUME/.kobo/KoboReader.sqlite" ]; then
  db_uri="file:$VOLUME/.kobo/KoboReader.sqlite?mode=ro&immutable=1"
  imported="$(sqlite3 "$db_uri" "SELECT count(*) FROM content WHERE ContentID LIKE 'file:///mnt/onboard/Matter/%.epub' AND ContentType = '6';")"
  collection="$(sqlite3 "$db_uri" "SELECT count(*) FROM ShelfContent WHERE ShelfName = 'Matter' AND _IsDeleted = 'false';")"
  echo "Kobo imported Matter EPUBs: $imported"
  echo "Matter collection entries: $collection"
fi
