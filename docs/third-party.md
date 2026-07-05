# Third-party Kobo components

The self-contained installer can merge these upstream KoboRoot payloads:

| Component | Version | URL | SHA-256 |
| --- | --- | --- | --- |
| NickelMenu | v0.6.0 | `https://github.com/pgaskin/NickelMenu/releases/download/v0.6.0/KoboRoot.tgz` | `322ff9aa863860e8f5f7e0b55cae561c54bf95983b9bce1d19819d1225d064af` |
| NickelDBus | 0.2.0 | `https://github.com/shermp/NickelDBus/releases/download/0.2.0/KoboRoot.tgz` | `9fdb3d16d0f43c1ea6f2f1264b10fcde1d72a674e55f12955de812d476eb5dc5` |

Fetch them with:

```sh
sh scripts/fetch-kobo-deps.sh
```

When present under `.vendor/`, `scripts/build-with-local-go.sh` includes them
in `dist/KoboRoot.tgz`.

