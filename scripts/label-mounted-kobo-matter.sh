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

DB="$VOLUME/.kobo/KoboReader.sqlite"
if [ ! -f "$DB" ]; then
  echo "KoboReader.sqlite not found: $DB"
  exit 1
fi

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "sqlite3 is required on this computer."
  exit 1
fi

now="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
shelf_id="$(uuidgen | tr '[:upper:]' '[:lower:]')"
backup_dir="$VOLUME/.adds/matter/db-backups"
backup="$backup_dir/KoboReader.before-matter-collection.$(date -u '+%Y%m%dT%H%M%SZ').sqlite"

mkdir -p "$backup_dir"
cp "$DB" "$backup"

sqlite3 "$DB" <<SQL
BEGIN;
INSERT INTO Shelf (
  CreationDate,
  Id,
  InternalName,
  LastModified,
  Name,
  Type,
  _IsDeleted,
  _IsVisible,
  _IsSynced,
  _SyncTime,
  LastAccessed
)
SELECT
  '$now',
  '$shelf_id',
  'Matter',
  '$now',
  'Matter',
  'UserTag',
  'false',
  'true',
  'false',
  '$now',
  '$now'
WHERE NOT EXISTS (
  SELECT 1 FROM Shelf WHERE Name = 'Matter'
);

UPDATE Shelf
SET
  InternalName = 'Matter',
  LastModified = '$now',
  Type = 'UserTag',
  _IsDeleted = 'false',
  _IsVisible = 'true'
WHERE Name = 'Matter';

DELETE FROM ShelfContent
WHERE ShelfName = 'Matter'
  AND ContentId NOT IN (
    SELECT ContentID
    FROM content
    WHERE ContentID LIKE 'file:///mnt/onboard/Matter/%.epub'
      AND ContentType = '6'
  );

INSERT OR REPLACE INTO ShelfContent (
  ShelfName,
  ContentId,
  DateModified,
  _IsDeleted,
  _IsSynced
)
SELECT
  'Matter',
  ContentID,
  '$now',
  'false',
  'false'
FROM content
WHERE ContentID LIKE 'file:///mnt/onboard/Matter/%.epub'
  AND ContentType = '6';
COMMIT;
SQL

count="$(sqlite3 "$DB" "SELECT count(*) FROM ShelfContent WHERE ShelfName = 'Matter' AND _IsDeleted = 'false';")"
echo "Matter collection now contains $count imported EPUB(s)."
echo "Database backup: $backup"
