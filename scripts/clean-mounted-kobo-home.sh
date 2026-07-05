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
  echo "No mounted Kobo volume found. Connect the Kobo over USB first."
  exit 1
fi

DB="$VOLUME/.kobo/KoboReader.sqlite"
if [ ! -f "$DB" ]; then
  echo "KoboReader.sqlite not found: $DB"
  exit 1
fi

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "sqlite3 is required on this computer."
  exit 1
fi

backup_dir="$VOLUME/.adds/matter/db-backups"
backup="$backup_dir/KoboReader.before-home-cleanup.$(date -u '+%Y%m%dT%H%M%SZ').sqlite"
mkdir -p "$backup_dir"
cp "$DB" "$backup"

before="$(sqlite3 "$DB" "
SELECT count(*)
FROM Activity
WHERE Enabled = 'true'
  AND Type IN (
    'Bookstore',
    'NewReleases',
    'Recommendations',
    'RelatedItems',
    'Top50',
    'TopPicksTab',
    'WhatsNew'
  );
")"

sqlite3 "$DB" <<'SQL'
BEGIN;
UPDATE Activity
SET Enabled = 'false'
WHERE Type IN (
  'Bookstore',
  'NewReleases',
  'Recommendations',
  'RelatedItems',
  'Top50',
  'TopPicksTab',
  'WhatsNew'
);
COMMIT;
SQL

after="$(sqlite3 "$DB" "
SELECT count(*)
FROM Activity
WHERE Enabled = 'true'
  AND Type IN (
    'Bookstore',
    'NewReleases',
    'Recommendations',
    'RelatedItems',
    'Top50',
    'TopPicksTab',
    'WhatsNew'
  );
")"

echo "Disabled Kobo home store/recommendation widgets: $before -> $after active row(s)."
echo "Database backup: $backup"
echo "Eject and reboot the Kobo for the home screen to rebuild."
