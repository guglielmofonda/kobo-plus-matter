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
CONF="$VOLUME/.kobo/Kobo/Kobo eReader.conf"

if [ ! -f "$DB" ]; then
  echo "KoboReader.sqlite not found: $DB"
  exit 1
fi

if [ ! -f "$CONF" ]; then
  echo "Kobo eReader.conf not found: $CONF"
  exit 1
fi

if ! command -v sqlite3 >/dev/null 2>&1; then
  echo "sqlite3 is required on this computer."
  exit 1
fi

stamp="$(date -u '+%Y%m%dT%H%M%SZ')"
backup_dir="$VOLUME/.adds/matter/db-backups"
config_backup_dir="$VOLUME/.adds/matter/config-backups"
mkdir -p "$backup_dir" "$config_backup_dir"

db_backup="$backup_dir/KoboReader.before-store-promo-cleanup.$stamp.sqlite"
conf_backup="$config_backup_dir/Kobo_eReader.before-store-promo-cleanup.$stamp.conf"
cp "$DB" "$db_backup"
cp "$CONF" "$conf_backup"

before_activity="$(sqlite3 "$DB" "
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

before_kobo_plus_groups="$(sqlite3 "$DB" "SELECT count(*) FROM KoboPlusAssetGroup;")"
before_kobo_plus_assets="$(sqlite3 "$DB" "SELECT count(*) FROM KoboPlusAssets;")"
before_subscription_products="$(sqlite3 "$DB" "SELECT count(*) FROM SubscriptionProducts;")"

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
DELETE FROM KoboPlusAssets;
DELETE FROM KoboPlusAssetGroup;
DELETE FROM SubscriptionProducts;
UPDATE user
SET Subscription = 0,
    NewUserPromoCurrency = '',
    NewUserPromoValue = 0
WHERE Subscription != 0
   OR NewUserPromoCurrency IS NOT ''
   OR NewUserPromoValue != 0;
COMMIT;
SQL

tmp_conf="$CONF.tmp.$stamp"
awk '
BEGIN {
  in_general = 0
  in_application = 0
  in_onestore = 0
}
/^\[/ {
  in_general = ($0 == "[General]")
  in_application = ($0 == "[ApplicationPreferences]")
  in_onestore = ($0 == "[OneStoreServices]")
}
in_application && /^KoboPlusPromoShown=/ {
  print "KoboPlusPromoShown=true"
  next
}
in_onestore && /^featured_list=/ {
  print "featured_list="
  next
}
in_onestore && /^featured_lists=/ {
  print "featured_lists="
  next
}
in_onestore && /^kobo_subscriptions_enabled=/ {
  print "kobo_subscriptions_enabled=False"
  next
}
in_onestore && /^product_recommendations=/ {
  print "product_recommendations="
  next
}
in_onestore && /^store_home=/ {
  print "store_home="
  next
}
in_onestore && /^subs_landing_page=/ {
  print "subs_landing_page="
  next
}
in_onestore && /^subs_management_page=/ {
  print "subs_management_page="
  next
}
in_onestore && /^subs_plans_page=/ {
  print "subs_plans_page="
  next
}
in_onestore && /^subs_purchase_buy_templated=/ {
  print "subs_purchase_buy_templated="
  next
}
in_onestore && /^subscription_publisher_price_page=/ {
  print "subscription_publisher_price_page="
  next
}
in_onestore && /^taste_profile=/ {
  print "taste_profile="
  next
}
in_onestore && /^user_recommendations=/ {
  print "user_recommendations="
  next
}
{ print }
' "$CONF" > "$tmp_conf"
mv "$tmp_conf" "$CONF"

after_activity="$(sqlite3 "$DB" "
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

after_kobo_plus_groups="$(sqlite3 "$DB" "SELECT count(*) FROM KoboPlusAssetGroup;")"
after_kobo_plus_assets="$(sqlite3 "$DB" "SELECT count(*) FROM KoboPlusAssets;")"
after_subscription_products="$(sqlite3 "$DB" "SELECT count(*) FROM SubscriptionProducts;")"

echo "Disabled Kobo home store/recommendation Activity rows: $before_activity -> $after_activity active row(s)."
echo "Cleared Kobo Plus promo groups: $before_kobo_plus_groups -> $after_kobo_plus_groups."
echo "Cleared Kobo Plus promo assets: $before_kobo_plus_assets -> $after_kobo_plus_assets."
echo "Cleared subscription products: $before_subscription_products -> $after_subscription_products."
echo "Set Kobo Plus/store recommendation service URLs to disabled/blank in Kobo eReader.conf."
echo "Database backup: $db_backup"
echo "Config backup: $conf_backup"
echo "Eject and reboot the Kobo for the home screen to rebuild."
