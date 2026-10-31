# Architecture

## Current milestone: foundation

- Core: Go HTTP service with liveness, readiness and a public, non-sensitive status endpoint.
- Web: single-origin React application; the web gateway forwards /api to Core.
- Media planning: pure Go decision logic to decide Direct Play, remux or selective transcode. It does not probe, download or play media.
- Persistence: PostgreSQL 17 schema and pgx pool support users, sessions, settings, and future progress. No production database is connected yet; sqlc generation is pending.
- Authentication uses Argon2id, revocable sessions and admin RBAC. Endpoints fail closed until DB migrations and an approved HTTP/HTTPS origin are configured. Actual media routes do not exist.
- YuTheme is imported as an external dependency; do not duplicate or override its palette.

## Target data flow

Browser or TV -> Core (authorize session and resolve source) -> media planner (probe + client capabilities) -> direct authorized response OR FFmpeg media worker -> protected HLS segments.

The media worker will have strict CPU/memory/task quotas, cancellation, TTL, and bounded tmpfs. Media links will use scoped, expiring access controls. The core must enforce host allowlists and public-address validation across DNS resolutions and redirects for arbitrary remote sources.

## Compatibility

Direct Play takes precedence. Remux copies supported audio/video codecs into an accepted container. Selective transcode converts only unsupported tracks, then full transcode if needed. Actual FFmpeg arguments must be derived from probed metadata and target capabilities. Reproducible playback tests are required before production enablement.

## Deployment topology

Two containers in the foundation: Core and Web. The web gateway binds to a high host port on all interfaces; no firewall or DNS changes are made. Media worker and PostgreSQL integration are intentionally deferred to avoid deploying a non-functional mock player. Support both native Go binaries and Compose; existing third-party media services are never modified.

## Roadmap

1. Foundation: endpoints, web shell, media decision logic and CI.
2. PostgreSQL schema, account authentication, RBAC and admin settings API.
3. Secure Stremio source integration and metadata lookup.
4. Direct Play with Range and authenticated streaming sessions.
5. FFprobe and isolated FFmpeg worker with progressive HLS, remux and transcode.
6. Browser/Android/iOS/TV playback tests including seek, restart, audio, subtitles and end-of-media.
7. IPTV, guides, and richer media discovery.

No milestone is complete until its specific tests pass.

## Network transport model

YuFlix supports HTTPS and explicitly opted-in HTTP access. Approved origins are exact, installation-specific strings, and the Core never derives trust from an arbitrary request Host. Sessions are bound to the HTTP or HTTPS transport and use separate cookies. HTTPS retains secure __Host-prefixed cookies. HTTP lacks confidentiality and must be limited to trusted networks.

The bundled Core trusts X-Forwarded-Proto only from its private Caddy gateway, which overwrites the header. Native Core deployments default to interpreting direct requests as HTTP unless TLS is terminated by the process. Custom reverse proxy configurations require a validated trusted proxy boundary.

## Current user experience

React Router supplies three real route boundaries: public login, authenticated home and admin-only configuration. The legacy combined diagnostic page and login form have been replaced. The app restores sessions from the backend and never stores session tokens or user passwords in browser storage. The authenticated home does not claim media playback capability. Details are in docs/web-ui.md.
