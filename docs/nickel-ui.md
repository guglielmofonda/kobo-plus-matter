# Nickel UI notes

## Matter labels

Matter-generated EPUBs include `Matter` as publisher, subject, source, and
collection metadata. The sync state records an EPUB metadata version so existing
articles are regenerated when the metadata contract changes.

For already imported articles, use:

```sh
sh dist/label-mounted-kobo-matter.sh
```

This creates a visible `Matter` Kobo collection from imported
`file:///mnt/onboard/Matter/*.epub` content rows and backs up
`.kobo/KoboReader.sqlite` first.

## Home and navigation UI

NickelMenu can add menu commands but cannot remove Kobo's built-in bottom
navigation item for Discover or replace the home screen recommendation tile.

The maintained kobopatch patch repository has a `4.38.21908` patch set with
home-screen layout tweaks such as hiding the footer row. This device is running
`4.38.23697`, and there is no published patch set for that exact firmware in the
checked repository. Applying binary patches across unmatched firmware builds is
not part of the default installer path.

For this exact `4.38.23697` firmware, `scripts/build-home-cleanup-patch.sh`
builds an optional Nickel patch at `dist/KoboRoot.home-cleanup.tgz` plus a
restore package at `dist/KoboRoot.home-cleanup-restore.tgz`. The patch hides the
top-right recommendation widget, hides the lower home-screen row containing the
Activity/Browse Kobo and SmartLink recommendation cards, and removes Kobo
Plus/wishlist/points SmartLinks from rotation. Install it only after confirming
the safer database/config cleanup was not enough:

```sh
sh dist/install-home-cleanup-to-mounted-kobo.sh
```

If it needs to be rolled back while the Kobo still mounts:

```sh
sh dist/install-home-cleanup-restore-to-mounted-kobo.sh
```

The safe current behavior is:

- Matter articles sync as normal EPUBs.
- Matter EPUBs are labeled with Matter metadata.
- A host-side helper can place imported Matter EPUBs into a visible `Matter`
  collection.
- A host-side helper can disable Kobo Shop/recommendation home widgets stored in
  the local Activity table.
- A host-side helper can also clear cached Kobo Plus promo assets and disable
  store recommendation endpoints in the local Kobo config.
- An optional firmware-matched Nickel patch can remove additional home-screen
  store surfaces, with a matching restore package.
- Kobo's own home recents can surface the Matter articles after import.

Replacing the store recommendation tile with a custom Matter widget would
require a firmware-matched Nickel patch or a separate custom launcher UI.
