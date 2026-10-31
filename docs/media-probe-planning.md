# Local FFprobe inspection and HLS planning

This is an **internal development milestone**, not a playable HLS endpoint or
an enabled transcode service.

## FFprobe inspection

- The Go media package parses bounded FFprobe JSON to identify a container,
  duration, video resolution, video/audio/subtitle codec, language, default
  and forced dispositions, audio channels, and the video transfer characteristic.
- Metadata such as source filenames, private URLs, arbitrary tags and stream
  extradata is **not returned** in the structured result.
- Preferred audio uses Brazilian Portuguese, then Portuguese, then the file's
  default track. Subtitles default to Brazilian Portuguese or Portuguese only
  when such a track exists. All other tracks remain discoverable; this is not
  yet a browser track selector.
- FFprobeFile only accepts regular files inside a specifically approved local
  root. Paths and symlinks are resolved and checked; remote URLs are rejected.
  The subprocess is bounded by context cancellation, an eight-second timeout,
  and a one-megabyte output limit. FFprobe receives a file-only protocol
  allowlist. No remote provider address or authentication token is passed.
- For untrusted streams, the eventual Media Engine must use an isolated worker
  with narrow mounts and verified remote input transport, never an unrestricted
  ffprobe or ffmpeg invocation against a provider-controlled URL.

## Typed FFmpeg HLS command planning

- HLSArguments accepts an explicit local input, playlist/segment paths, absolute
  media stream indexes, seek offset and a previously chosen plan. It builds an
  argument array, not a shell command. Nothing executes automatically.
- **Remux:** copy compatible video and audio without recoding.
- **Audio-only transcode:** copy video, encode audio with AAC.
- **Video transcode:** use software H.264, with one FFmpeg thread and an
  ultrafast preset; copy compatible audio or encode AAC as needed.
- Existing HDR transfers such as PQ and HLG are rejected for video transcoding
  until an explicit, tested tone-mapping pipeline exists.
- HLS segments are prepared as MPEG-TS with bounded sliding playlist options.
  These arguments alone do not guarantee complete playback, live startup, seek,
  stable segments or appropriate output retention.
- Subtitle selection, conversion to WebVTT, discontinuities, partial segments,
  resource quotas/cgroups, worker queue, tmpfs limits, cleanup, authorization of
  playlists/segments, hardware acceleration and device profiles are **not yet
  implemented**.

## Validation

The LAB generated a short original MPEG-4/AAC fixture locally and used
installed FFmpeg 6.1.1/FFprobe 6.1.1 to check real codec detection and creation
of a short HLS segment plus playlist. Go unit tests also exercise priority,
path rejection, malformed metadata, mode-specific stream copying and HDR refusal.
Neither test proves end-to-end playback from any external addon.

The active production Core intentionally does not include FFmpeg/FFprobe in
its runtime image and continues to use the pre-existing, authorized progressive
MP4 relay. Introducing a dedicated containerized FFmpeg worker is required
before these internal helpers can serve real media.

Primary references:
- https://ffmpeg.org/ffprobe.html
- https://ffmpeg.org/ffmpeg-formats.html#hls-2
- https://developer.mozilla.org/en-US/docs/Web/Media/Guides/Formats

The local-only HLS worker now has a bounded cache policy, real-time FFmpeg pacing, per-session cleanup, cancellation, orphan cleanup and manual/automatic RAM/disk budgets. This is not a publicly accessible player endpoint or an enabled worker service. See [HLS cache policy](media-cache.md).
