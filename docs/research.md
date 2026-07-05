# Research notes

## Matter API

Matter's public API uses bearer-token authentication at:

```text
https://api.getmatter.com/public/v1/
```

The API requires Matter Pro. Useful endpoints:

- `GET /me` verifies the token.
- `GET /items?status=queue&limit=100` lists queued library items.
- `GET /items/{id}?include=markdown` returns full item metadata plus parsed
  markdown.

Rate limits relevant to this project:

- read: 120 requests/minute
- markdown content: 20 requests/minute
- burst: 5 requests/second

The sync client should therefore page in batches, cache item state locally, and
throttle markdown fetches.

To mirror `SYNC_STATUS=queue` correctly, the client lists the full configured
Matter view each run and compares it with local state. This lets it remove
generated EPUBs when items are archived or otherwise leave the queue, while
`MAX_ITEMS_PER_SYNC` only limits expensive markdown body fetches.

## Kobo integration

Kobo install packages use the `KoboRoot.tgz` mechanism and are copied to the
hidden `.kobo` folder on the Kobo USB volume.

NickelMenu provides menu entries and can:

- run shell commands
- connect to known Wi-Fi networks
- force a book rescan or full USB import cycle

NickelDBus provides a D-Bus interface and a `qndb` CLI that can trigger a full
book rescan from scripts:

```sh
qndb -t 30000 -s pfmDoneProcessing -m pfmRescanBooksFull
```

The bundled NickelDBus runtime also exposes `wfmConnectWirelessSilently`, which
the daemon uses to ask Nickel to connect to a known Wi-Fi network before waiting
for Matter's API host.

The current package merges the official NickelMenu v0.6.0 and NickelDBus 0.2.0
KoboRoot payloads into the generated installer.

## Compatibility gates

- NickelMenu does not currently support Kobo firmware 5.x.
- NickelMenu's documented support starts at firmware 4.6+.
- The generated sync binary targets Linux ARM with `GOARM=6` for broad Kobo
  compatibility.
