# Goal

Build a drag-and-drop Kobo installer that adds automatic Matter article sync
while preserving the normal Kobo reading experience.

## Target outcome

- A Kobo install artifact at `dist/KoboRoot.tgz`.
- The Kobo continues to boot into Nickel and read normal EPUBs as before.
- Matter queue articles are fetched over Wi-Fi and written as EPUB files under
  `/mnt/onboard/Matter`.
- A user-visible NickelMenu entry can force sync, show status, connect Wi-Fi,
  and trigger a book import.
- When NickelDBus is present, the sync client triggers a Nickel library rescan
  after writing new EPUBs.

## Architecture decision

Use stock Kobo firmware plus NickelMenu/NickelDBus, not KOReader as the primary
reader and not a replacement OS.

Reasons:

- Matter already exposes the article body as markdown through the public API.
- Kobo can import ordinary sideloaded EPUB files into the standard library.
- NickelMenu is designed to coexist with Nickel and run custom commands.
- NickelDBus gives scripts a supported D-Bus surface for rescans and Wi-Fi
  signals.
- KOReader is valuable as an alternate reader, but it changes the reading
  surface rather than solving the sync/import problem directly.

## Verified device inputs

- Device version observed from `.kobo/version`:
  `N70977L010117,3.0.35+,4.38.23697,3.0.35+,3.0.35+,00000000-0000-0000-0000-000000000373`.
- The model family is `N709`, a Kobo Aura ONE generation device.
- Matter API token was seeded into `/mnt/onboard/.adds/matter/config.env` on the
  device, not committed into the repo.
- The installer was consumed successfully by the Kobo updater, and the visible
  onboard files verified after reboot.

## Remaining verification

- Let one full Matter sync complete on-device without USB reconnect interrupting
  it.
- Reconnect the Kobo and verify `last_sync_finished_at`, EPUB count, and the
  visible `Matter` collection membership.
