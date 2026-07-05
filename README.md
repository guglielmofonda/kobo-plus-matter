# Kobo plus Matter

This project builds a Kobo install package that syncs Matter queue items into
normal EPUB files on the Kobo filesystem, so they can be imported and read with
Kobo's stock Nickel reader.

Website: [kobo-plus-matter.vercel.app](https://kobo-plus-matter.vercel.app)
Source: [github.com/guglielmofonda/kobo-plus-matter](https://github.com/guglielmofonda/kobo-plus-matter)

The intended install path is deliberately conservative:

- keep Kobo's stock OS and reader
- use the Matter public API to fetch queued article markdown
- generate EPUB files under `/mnt/onboard/Matter`
- remove generated EPUBs when their Matter items leave the configured view
- bundle NickelMenu for visible controls
- bundle NickelDBus to request Wi-Fi autoconnect and trigger a library rescan
  after sync

## Current recommendation

Do not replace Kobo's operating system for this. KOReader is a capable alternate
reader, but it is heavier than needed if the goal is to keep reading EPUBs in
the normal Kobo library. NickelMenu is the right user-facing integration point,
and NickelDBus is the cleanest way for a background process to ask Nickel to
rescan newly created books.

## Build

This repo needs Go to build the Kobo ARM binary.

```sh
sh scripts/fetch-kobo-deps.sh
GOOS=linux GOARCH=arm GOARM=6 CGO_ENABLED=0 go build \
  -o build/matter-kobo-sync ./cmd/matter-kobo-sync
python3 scripts/package.py --binary build/matter-kobo-sync \
  --merge-koboroot .vendor/NickelMenu-v0.6.0-KoboRoot.tgz \
  --merge-koboroot .vendor/NickelDBus-0.2.0-KoboRoot.tgz
```

To build a private package with your Matter token included in
`/mnt/onboard/.adds/matter/config.env`, run:

```sh
MATTER_TOKEN=mat_your_token_here python3 scripts/package.py \
  --binary build/matter-kobo-sync --include-token
```

The installer will be written to `dist/KoboRoot.tgz`.
The build also writes `dist/install-to-mounted-kobo.sh`, a Mac helper that
copies the package to a connected Kobo's hidden `.kobo` folder.
It also writes `dist/verify-mounted-kobo.sh`, which checks a reconnected Kobo
after reboot/install.
`dist/seed-matter-token.sh` can add the Matter token later without restaging the
installer, `dist/update-mounted-kobo.sh` refreshes an already-installed Kobo
with the latest visible Matter files, `dist/label-mounted-kobo-matter.sh`
creates a visible `Matter` collection from imported articles,
`dist/clean-mounted-kobo-home.sh` disables local Kobo store/recommendation home
widgets, `dist/disable-mounted-kobo-store-promos.sh` clears cached Kobo Plus
promo assets and disables store recommendation endpoints,
`dist/install-home-cleanup-to-mounted-kobo.sh` stages the optional
firmware-matched Nickel patch which hides the distracting home recommendation
row,
`dist/install-home-cleanup-restore-to-mounted-kobo.sh` stages its rollback
package,
`dist/finalize-mounted-kobo.sh` runs the post-sync USB verification pass, and
`dist/wait-for-kobo.sh` waits for reconnect then verifies.

## Install

Kobo firmware installs packages from `.kobo/KoboRoot.tgz`, not from the visible
root folder. Copy `dist/KoboRoot.tgz` to the hidden `.kobo` folder on the Kobo
USB volume, eject the device, and let it reboot.

If the Kobo is connected to the laptop, the helper can do that copy:

```sh
sh dist/install-to-mounted-kobo.sh
```

To copy the installer and pre-seed the Matter token in one step:

```sh
MATTER_TOKEN=mat_your_token_here sh dist/install-to-mounted-kobo.sh
```

After the Kobo reboots and is reconnected:

```sh
sh dist/verify-mounted-kobo.sh
```

To add the Matter token after install:

```sh
MATTER_TOKEN=mat_your_token_here sh dist/seed-matter-token.sh
```

To wait for the Kobo and run verification automatically:

```sh
sh dist/wait-for-kobo.sh
```

After install:

1. Connect the Kobo to Wi-Fi using Kobo's normal settings.
2. Add your Matter API token to `/mnt/onboard/.adds/matter/config.env` if it was
   not embedded at build time.
3. Use the NickelMenu `Matter Sync` item for a manual first sync.
4. The bundled NickelDBus install lets the sync client trigger a full library
   rescan after new EPUBs are written.

See [docs/install.md](docs/install.md) for exact copy, config, recovery, and
auto-sync notes.
See [docs/nickel-ui.md](docs/nickel-ui.md) for what is safe to change in the
stock Kobo UI and what requires firmware-matched Nickel patching.

## Recovery

Remove `/mnt/onboard/.adds/matter` to remove the sync client and generated
state. Remove `/mnt/onboard/.adds/nm/matter` to remove the NickelMenu entries.

If an experimental autostart hook was installed and you need to disable it,
delete `/mnt/onboard/.adds/matter/config.env` or set:

```sh
AUTO_SYNC_ENABLED=0
```
