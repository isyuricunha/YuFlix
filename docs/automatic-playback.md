# Automatic playback and bounded metadata cache

YuFlix separates the viewer's experience from the administrator's source
configuration. This milestone supports only progressive MP4 over approved
public HTTPS URLs with byte-range support, including the previously configured
private egress path.

## Viewer flow

- The title screen has a single **Watch now** action, not an addon/source list.
- POST /api/v1/media/auto accepts a validated movie or episode ID and an
  optional retry attempt (0 through 7). It never accepts remote media URLs.
- Eligible addon discovery runs with up to three concurrent requests, each
  respecting its own timeout. Results are consumed in stable priority order,
  starting with the lowest admin-configured numeric priority.
- Sources that fail discovery or bounded Range probes are skipped. A successful
  probe yields an opaque, user-bound, transport-bound, expiring session ticket.
- The browser can request the next candidate following a playback error by
  sending the returned nextAttempt index; no provider names or URLs are shown.
  A 206 and MP4 signature are not a guarantee of end-to-end playback.
- Episodes may request the actual next episode from the title metadata when
  playback ends. Browser autoplay restrictions can still require user action.

## Administration

- Each stream addon stores priority (1 to 1000; default 500) and source timeout
  (1000 to 10000 milliseconds; default 5000) in existing source_configs JSONB.
  Equal priorities preserve original source order.
- Direct, private egress and automatic network routing remain independent
  configuration options. Manual source APIs are admin-only.
- Provider URLs and private media paths are never included in viewer responses.

## Persistent catalog snapshots

Snapshots are persisted in PostgreSQL 17 using additive migration 0003.

- **Off** skips metadata caching and deletes saved snapshots for the selected
  source or all sources.
- **On demand** caches only catalog pages that users actually request.
- **Limited precache** queues the first two eligible catalog categories on
  configuration change. No mass catalog scans are performed.
- Snapshots include up to 25 sanitized metadata items per catalog and at most
  1 MiB of JSON per snapshot. Playback URLs are never part of the data.
- Fresh snapshots are reused for six hours. Between 6 and 48 hours they are
  served immediately while a background fetch tries to update them.
- Refresh workers are limited to two concurrent fetches and 32 pending jobs.
  Erroring addons do not invalidate existing snapshots.
- Cache is shared across accounts, but watch history and session tickets are
  not. Source deletion cascades to its persisted snapshots.

The initial homepage still shows at most four catalog rails. Additional pages,
scheduled idle precache, metadata pagination, subtitle/audio track selection,
FFprobe-driven remux and bounded transcoding are separate milestones.

## Operational safety

Apply new migrations only after a verified backup of the dedicated database.
A test passing with an isolated PostgreSQL server does not prove real media
playback on a browser, Android or iOS device. Before deployment, run
Go race tests, isolated PG integration, TypeScript, Vitest, production Vite
build and real device playback checks. Updating the Core can interrupt active
video sessions and must be scheduled accordingly.
