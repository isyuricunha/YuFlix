# Media Engine, first playback milestone

YuFlix has a native Stremio stream discovery interface and a deliberately narrow, secure Direct Play implementation for compatible progressive MP4 files.

## User workflow

1. Administrators add a public stream addon under Administration > Reprodução. The entire configured HTTPS manifest URL is accepted, including URL-encoded configuration before /manifest.json.
2. Visit a catalog title. For series, select a specific episode.
3. Select Buscar fontes. Only public HTTPS progressive MP4 sources that do not require proxy headers are shown.
4. Select Testar Direct Play. The Core fetches a short bounded HTTP byte range, checks HTTP 206 and MP4 file signature, and creates a user-bound expiring session.
5. The player accesses the Core over the existing installation origin using a temporary path and the same authenticated cookie.

Stream providers are shared among users; the sessions remain scoped to each user. The manifest URL (which can contain user-specific configuration) and the original stream address, including any query-string signature, is never returned to the browser or written in normal logs.

## Playback and limitations

This milestone supports only:
- Public HTTPS URLs ending in .mp4 or .m4v, optionally including a signed query string.
- Byte-range streaming with Content-Range, HTTP 206 and seek requests.
- Go standard library streaming relay without disk buffering, FFmpeg invocation or transcode.
- One source selected explicitly by the user; opaque candidate bindings detect provider reordering, and failure does not delete other sources.
- Concurrent verification and stream session limits; source sessions are in memory, valid up to six hours, require the creator's authenticated account and matching HTTP or HTTPS transport, and are revoked when the provider is removed.

This milestone deliberately does not support:
- HLS, DASH, MKV, torrents or providers that require custom request headers or authenticated addon manifests.
- Full codec negotiation, FFprobe of remote sources, audio/subtitle tracks, HDR handling or transcoding.
- Automatic failover, progress storage, TV client compatibility or per-device profiles.
- A claim that an HTTP 206, MP4 signature or short playback establishes complete watchability.

The server validates DNS/IP addresses for every new public HTTPS connection and redirect, blocking private addresses, CGNAT/Tailscale, loopback, link-local and special transition ranges. The proxy forwards only restricted video headers, and does not forward upstream cookies, locations or provider credentials.

The browser uses the native video element. Direct Play compatibility is device specific; report when codecs cannot be decoded. Session state is memory-only, so a Core restart invalidates video session tickets, and viewers must select their source again. This is explicitly a preliminary media engine, not a finished playback guarantee.

The browser player works over HTTP if the operator has explicitly enabled that trusted origin. HTTPS is strongly recommended because HTTP media and login traffic are not encrypted.

## Verification

- go test -race ./... for stream protocol, security, sessions and Range behavior.
- PostgreSQL 17 isolated integration tests for shared source permissions.
- pnpm test and pnpm build for web flows and source selection.
- Chromium desktop/mobile checks using an original synthetic H.264/AAC MP4 for start, Range/206, seek and end. These tests use mocked media API responses and do not prove a real external provider works.

Before considering the Media Engine production complete, run real source sessions on Chromium/Firefox, Android, iOS and TV for playback start, seek, track switching, end, reconnect and resume, and benchmark actual memory/CPU use under concurrency.

## Primary references

- https://stremio.github.io/stremio-addon-sdk/protocol.html
- https://stremio.github.io/stremio-addon-sdk/api/responses/stream.html
- https://www.ffmpeg.org/ffprobe.html
- https://developer.mozilla.org/en-US/docs/Web/HTTP/Range_requests
- https://developer.mozilla.org/en-US/docs/Web/Media/Guides/Formats
- https://caddyserver.com/docs/caddyfile/directives/reverse_proxy
