# Architecture

## Current milestone: foundation

- Core: Go HTTP service with liveness, readiness and a public, non-sensitive status endpoint.
- Web: single-origin React application; the web gateway forwards /api to Core.
- Media planning: pure Go decision logic to decide Direct Play, remux or selective transcode. It does not probe, download or play media.
- Persistence: PostgreSQL 17 via pgx and sqlc is planned; no database is connected in this milestone.
- Authentication, admin configuration and actual media routes are not enabled yet. Sensitive endpoints must never be introduced unprotected.
- YuTheme is imported as an external dependency; do not duplicate or override its palette.

## Target data flow

Browser or TV -> Core (authorize session and resolve source) -> media planner (probe + client capabilities) -> direct authorized response OR FFmpeg media worker -> protected HLS segments.

The media worker will have strict CPU/memory/task quotas, cancellation, TTL, and bounded tmpfs. Media links will use scoped, expiring access controls. The core must enforce host allowlists and public-address validation across DNS resolutions and redirects for arbitrary remote sources.

## Compatibility

Direct Play takes precedence. Remux copies supported audio/video codecs into an accepted container. Selective transcode converts only unsupported tracks, then full transcode if needed. Actual FFmpeg arguments must be derived from probed metadata and target capabilities. Reproducible playback tests are required before production enablement.

## Deployment topology

Two containers in the foundation: Core and Web. The web gateway binds to loopback only. Media worker and PostgreSQL integration are intentionally deferred to avoid deploying a non-functional mock player. Support both native Go binaries and Compose; existing third-party media services are never modified.

## Roadmap

1. Foundation: endpoints, web shell, media decision logic and CI.
2. PostgreSQL schema, account authentication, RBAC and admin settings API.
3. Secure Stremio source integration and metadata lookup.
4. Direct Play with Range and authenticated streaming sessions.
5. FFprobe and isolated FFmpeg worker with progressive HLS, remux and transcode.
6. Browser/Android/iOS/TV playback tests including seek, restart, audio, subtitles and end-of-media.
7. IPTV, guides, and richer media discovery.

No milestone is complete until its specific tests pass.
