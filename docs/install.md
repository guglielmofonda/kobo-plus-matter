# Install guide

## Artifact

Use:

```text
dist/KoboRoot.tgz
```

The helper archive:

```text
dist/matter-kobo-usb-copy.zip
```

contains `.kobo/KoboRoot.tgz` for convenience, but a Kobo will not install a zip
file directly. The Kobo updater looks for `KoboRoot.tgz` in the hidden `.kobo`
folder.

## Copy to Kobo

Fast path from this repo:

```sh
sh dist/install-to-mounted-kobo.sh
```

That script finds the mounted Kobo volume and copies `dist/KoboRoot.tgz` to the
hidden `.kobo` updater folder.

To pre-seed the Matter token before reboot:

```sh
MATTER_TOKEN=mat_your_token_here sh dist/install-to-mounted-kobo.sh
```

Manual path:

1. Connect the Kobo over USB.
2. Open the mounted Kobo volume.
3. Show hidden files if needed.
4. Copy `dist/KoboRoot.tgz` to:

   ```text
   KOBOeReader/.kobo/KoboRoot.tgz
   ```

5. Eject the Kobo.
6. Let it reboot and install.

## Configure Matter

If the installer was built without `--include-token`, connect the Kobo over USB
again after install and edit:

```text
KOBOeReader/.adds/matter/config.env
```

If it does not exist yet, run `Matter Status` once from NickelMenu or copy this
sample yourself:

```text
KOBOeReader/.adds/matter/config.env.sample
```

Set:

```text
MATTER_TOKEN=mat_your_token_here
```

Matter API tokens are available from Matter settings and require a Matter Pro
account.

## Verify Install

After the Kobo reboots and you reconnect it over USB:

```sh
sh dist/verify-mounted-kobo.sh
```

The verifier checks the visible onboard files installed by the package and
reports whether a Matter token has been configured. It cannot inspect system
root files such as `/usr/bin/qndb` over normal USB mass storage, but the package
archive verifier checks those before staging.

The Matter client uses the bundled CA bundle at:

```text
KOBOeReader/.adds/matter/cacert.pem
```

This avoids relying on the Kobo firmware's root certificate store, which can be
too old for the Matter API TLS chain.

To wait for the Kobo to reconnect and then verify:

```sh
sh dist/wait-for-kobo.sh
```

To seed the Matter token after install:

```sh
MATTER_TOKEN=mat_your_token_here sh dist/seed-matter-token.sh
```

To update an already-installed Kobo with the latest visible Matter files without
restaging a root installer:

```sh
sh dist/update-mounted-kobo.sh
```

This preserves the existing `config.env` token and copies the current sync
binary, CA bundle, NickelMenu entries, and Matter scripts from the built
artifact.

To label imported articles with a visible Kobo collection after reconnecting the
Kobo over USB:

```sh
sh dist/label-mounted-kobo-matter.sh
```

The helper creates a `Matter` collection from the Matter EPUBs already imported
into Kobo's library database and stores a timestamped database backup under
`.adds/matter/db-backups/` first.

To hide Kobo store/recommendation widgets from the home screen:

```sh
sh dist/clean-mounted-kobo-home.sh
```

This disables local Activity rows for Kobo Shop, recommendations, top picks,
new releases, and what's-new widgets. It backs up `KoboReader.sqlite` first.
Eject and reboot the Kobo afterwards.

If Kobo Plus or Kobo Shop promotional cards still remain after that reboot:

```sh
sh dist/disable-mounted-kobo-store-promos.sh
```

This makes timestamped backups of `KoboReader.sqlite` and `Kobo eReader.conf`,
then clears cached Kobo Plus promo tables and disables store recommendation
endpoints in Kobo's local config. Eject and reboot the Kobo afterwards.

If those cards still remain visible on firmware `4.38.23697`, build and stage
the optional Nickel home cleanup patch:

```sh
sh scripts/build-home-cleanup-patch.sh
sh dist/install-home-cleanup-to-mounted-kobo.sh
```

This writes `dist/KoboRoot.home-cleanup.tgz` to the Kobo updater slot and also
copies `dist/KoboRoot.home-cleanup-restore.tgz` onto the device under
`.adds/matter/`. Eject and reboot the Kobo afterwards. To roll back while the
Kobo still mounts:

```sh
sh dist/install-home-cleanup-restore-to-mounted-kobo.sh
```

To run the full post-sync USB pass after reconnecting the Kobo:

```sh
sh dist/finalize-mounted-kobo.sh
```

This updates visible Matter files, verifies the install, refreshes the `Matter`
collection, and prints sync/import counts.

## First run

1. Connect the Kobo to Wi-Fi from normal Kobo settings.
2. Open NickelMenu.
3. Tap `Matter Sync`.
4. Tap `Matter Start Auto Sync` to start the background loop without waiting for
   the next reboot.

Generated articles are written to:

```text
KOBOeReader/Matter/
```

The sync folder mirrors the configured Matter view. With the default
`SYNC_STATUS=queue`, generated EPUBs are removed when the corresponding Matter
item leaves the queue.

## Automatic import

The package includes an experimental boot hook:

```text
/etc/rcS.d/S99matter-kobo
```

It starts the daemon once `/mnt/onboard` is mounted. The daemon syncs every
`SYNC_INTERVAL_MINUTES` while the Kobo is awake. With `AUTO_CONNECT_WIFI=1`, it
asks NickelDBus to connect to a known Wi-Fi network before each sync. It then
waits up to `NETWORK_WAIT_SECONDS` for the Matter API host to become reachable,
which helps when Wi-Fi is still connecting after wake or reboot.

If a sync fails because Wi-Fi or NickelDBus is not ready yet, the daemon retries
within five minutes instead of waiting for the full sync interval.

This package bundles NickelDBus, including the `qndb` CLI, so the daemon asks
Nickel to rescan books after writing new EPUBs. If NickelDBus fails to load on a
future firmware, articles are still written to disk, but Nickel may need the
manual `Matter Import Books` action or a reconnect cycle before they appear in
the library.

## Disable auto sync

Edit:

```text
KOBOeReader/.adds/matter/config.env
```

Set:

```text
AUTO_SYNC_ENABLED=0
```

You can also use the NickelMenu `Matter Stop Auto Sync` entry to stop the
current background process.

## Uninstall

Remove:

```text
KOBOeReader/.adds/matter
KOBOeReader/.adds/nm/matter
```

The root boot hook is harmless without `/mnt/onboard/.adds/matter/run-daemon.sh`,
but a future uninstall package should remove:

```text
/etc/init.d/matter-kobo
/etc/rcS.d/S99matter-kobo
```
