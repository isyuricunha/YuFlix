# Media Engine: bounded HLS cache policy

The media cache policy lives in PostgreSQL 17 under the admin-only
`media.cache_policy` setting. It is independent of the persistent **catalog
metadata** cache. Playback source URLs, signed tokens, user media, subtitles
and login data are **never** stored as part of this setting.

## Default policy

| Setting | Automatic default |
| --- | --- |
| Policy | Automatic |
| Storage | RAM-backed tmpfs, no silent spill to disk |
| Maximum | Up to 128 MiB, limited further by worker-reported free memory |
| Concurrent jobs | 1 |
| Segment duration | 3 seconds |
| Idle timeout | 90 seconds |
| Cleanup grace | 15 seconds |

Automatic mode leaves at least 256 MiB and 75% of free RAM untouched (using
the more conservative available amount). When less than 64 MiB can be
allocated, the worker refuses to start HLS instead of filling memory or
switching to SSD without approval.

Manual mode supports RAM or a dedicated disk volume, hard policy bounds of
64–4096 MiB, 1–4 jobs, 2–10 second segments, 30–600 second idle time and
0–180 second cleanup grace. The disk mode requires at least 1 GiB of free
space above the configured budget. **Choosing a policy does not promise that
the LAB hardware can sustain the resulting transcode workload.**

## Lifecycle and cleanup

1. A dedicated **local-only** worker receives an explicitly approved
   file/track/plan, not a remote URL or private source token.
2. The worker creates a private `job-*` directory in the selected cache
   root, with restrictive file permissions.
3. FFmpeg is paced to real time and writes segments atomically. Its HLS
   muxer removes segments that fall out of the sliding playlist, with a
   small retention window to avoid prematurely deleting data still
   requested by the client.
4. A watchdog checks cache consumption and last session access. If usage
   exceeds the configured budget, or no activity occurs before the idle
   timeout, the process is cancelled.
5. On normal finish, browser-initiated stop, timeout or failure, the job
   directory is deleted after a small grace period. Shutdown cancels jobs
   and deletes their outputs immediately.
6. Startup orphan cleanup removes only worker-created `job-*` folders
   from specifically reserved roots. Other directories remain untouched.

**Important:** filesystem monitoring can briefly overshoot the configured
limit between checks. RAM requires a size-limited tmpfs mount. Disk cache
requires an isolated directory/filesystem with capacity enforcement at the
deployment layer to provide a strict physical quota. Do not mount the
operating-system root or existing libraries as a worker cache directory.

## Current deployment state

The administration UI and policy API are implemented and persist settings.
The standalone, local-only Go HLS worker is tested with synthetic files,
but **is not yet exposed through a YuFlix playback route**.

The core runtime does not launch FFmpeg. A separate unprivileged container,
trusted internal transport, bounded source input, authorized playlist/segment
routing, device capability negotiation, and real playback/seek benchmarks
are still required before activating end-to-end HLS. Until then, the admin
endpoint returns `active: false` and the player keeps using the existing
progressive MP4 Direct Play path.

Sources:

- https://www.ffmpeg.org/ffmpeg-formats.html
- https://jellyfin.org/docs/general/server/tasks/
- https://jellyfin.org/docs/general/post-install/transcoding/
