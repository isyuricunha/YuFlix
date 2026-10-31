# Stremio catalogs in YuFlix

YuFlix reads public Stremio addon manifests, catalogs and metadata without depending on AIOStreams.

## Configure

As an administrator, open **Catálogos** and add a public HTTPS Stremio manifest. Sources are shared by all accounts. Example public provider:

    https://v3-cinemeta.strem.io/manifest.json

Operators can use the idempotent bootstrap command:

    docker compose run --rm --no-deps core add-public-catalog https://v3-cinemeta.strem.io/manifest.json

The app accepts public HTTPS manifests ending in /manifest.json, including percent-encoded configuration path segments with movie/series catalog feeds that do not require extra arguments. Catalogs show up to 25 items per shelf. Title details use the /meta resource. Catalog requests use a bounded 90-second in-memory cache and do not perform broad scans.

**No media playback is implemented.** Catalog metadata does not establish the existence or validity of playable video sources.

## Security and privacy

The backend validates each public HTTPS destination and pins the connection to validated public DNS addresses, rejecting private and Tailscale ranges, redirects to local networks, URL credentials, URL query parameters and ambiguous paths. Configuration embedded in the URL path is accepted but stored privately in the database and never exposed through the frontend API. The frontend does not receive the stored manifest URL.

Private Stremio addons, HTTP addons, IPTV, credential-bearing URLs, subtitles and media streams are deliberately outside this milestone. They need separate scoped secrets, network allowlists and actual playback verification.

Artwork images are public HTTPS URLs without query strings. The browser currently requests artwork directly from its public provider, revealing the viewer's IP to that provider. A constrained same-origin image proxy is a later privacy improvement. Catalog metadata may not be available in PT-BR.

## Official references

- https://stremio.github.io/stremio-addon-sdk/protocol.html
- https://stremio.github.io/stremio-addon-sdk/api/responses/manifest.html
- https://stremio.github.io/stremio-addon-sdk/api/responses/meta.html
- https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html

## Verification

Run go test -race ./... within services/core, pnpm test and pnpm build at repository root.
For an opt-in live upstream check, set YUFLIX_TEST_PUBLIC_ADDON=true and run go test -run TestPublicCinemetaIntegration -v ./internal/catalog.

Use a disposable PostgreSQL database for the SQL integration test, never production.

## Persistent metadata snapshots

Catalog pages are cached per source in PostgreSQL 17 using migration 0003. The admin can select Off, On demand or Limited precache globally or per source. Fresh snapshots live for six hours; snapshots up to 48 hours old are served while bounded background refresh runs. Playback URLs are never persisted. The first two eligible catalog categories may be warmed on selecting precache, without scanning the entire upstream catalog. See [automatic playback](automatic-playback.md).
