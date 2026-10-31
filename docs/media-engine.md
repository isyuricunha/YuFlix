# Media Engine, first playback milestone

YuFlix has a native Stremio stream discovery interface and a deliberately narrow, secure Direct Play implementation for compatible progressive MP4 files.

## User workflow

1. Administrators add a public stream addon under Administration > Reprodução. The entire configured HTTPS manifest URL is accepted, including URL-encoded configuration before /manifest.json.
2. Visit a catalog title. For series, select a specific episode.
3. Select Buscar fontes. Only public HTTPS MP4 sources and opaque extensionless HTTPS URLs that do not require proxy headers are shown. Extensionless candidates must pass a bounded MP4 signature and Range probe before a session is created.
4. Select Testar Direct Play. The Core fetches a short bounded HTTP byte range, checks HTTP 206 and MP4 file signature, and creates a user-bound expiring session.
5. The player accesses the Core over the existing installation origin using a temporary path and the same authenticated cookie.

Stream providers are shared among users; the sessions remain scoped to each user. The manifest URL (which can contain user-specific configuration) and the original stream address, including any query-string signature, is never returned to the browser or written in normal logs.

## Playback and limitations

This milestone supports only:
- Public HTTPS URLs ending in .mp4 or .m4v, or opaque extensionless HTTPS URLs when the byte-range response is verified as MP4. A signed query string is permitted; URLs are never exposed to clients.
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

## Troubleshooting upstream stream discovery

YuFlix distinguishes upstream HTTP denial (403), no streams returned, unsupported media formats, timeout and other network failures in the authenticated title view. Responses carry only provider display names and sanitized error codes. Manifest and signed stream URLs are never returned.

An addon manifest returning 200 only means registration succeeded. HTTP 403 for /stream/... is an upstream access refusal. AIOStreams can display cached or library-backed sources; its "(Your Media)" formatter label identifies library-backed media rather than proving that a public addon is returning fresh results.

## Optional private-network stream discovery egress

If a Stremio provider permits stream queries from a second, user-owned machine
but not the main YuFlix Core host, the operator can deploy the small YuFlix
egress role on that machine. This is an optional network transport, not an
AIOStreams dependency or a generic proxy.

- Egress binds **only** to an explicitly configured Tailscale IP (port 62582).
- Both machines must be in the same authorized Tailscale network.
- A randomly generated bearer secret authenticates requests. The secret and
  Tailscale address belong only to private bootstrap configuration, not Git.
- Set YUFLIX_EGRESS_TOKEN and YUFLIX_EGRESS_ALLOWED_HOSTS on the egress host.
- Set YUFLIX_STREAM_EGRESS_URL, YUFLIX_STREAM_EGRESS_TOKEN, and
  YUFLIX_STREAM_EGRESS_HOSTS on the Core host.
- Configure the egress using deploy/egress.compose.yaml with private
  YUFLIX_EGRESS_BIND_IP and YUFLIX_EGRESS_ENV_FILE values.

Only public HTTPS addon stream discovery requests for allowlisted hostnames are
accepted. Each URL goes through YuFlix's ordinary SSRF, DNS pinning, and
response-size validation. If public MP4 media URLs are also blocked from Core, the optional egress
role can validate and relay the exact previously discovered stream. Media
sessions are bound to the authenticated YuFlix account, with a bounded,
read-only streaming relay over Tailscale and no disk/full-file buffering.
Only URLs returned by an allowlisted addon and verified with HTTP Range/MP4
signature are eligible; the egress service cannot be used as an open proxy.
Concurrent media transfers are limited to six, and authorizations expire
after six hours.

The egress role has a small in-memory TTL cache, three concurrent requests,
restricted CPU/RAM, no persistent state, and returns only candidates to the
private Core over Tailscale encryption. If egress is not configured, YuFlix
behaves exactly as before. The relay should never be published on a public
network interface.

### Provider-owned media redirects

A public HTTPS MP4 source may return a 30x redirect to a public HTTP video
endpoint, including a nonstandard destination port. YuFlix can follow these
media redirects only after revalidating the destination hostname and resolving
all DNS results to public IP addresses. Private IP destinations, redirects
with user credentials, localhost/special-use names and excessive redirect
chains remain blocked. Credentials, user cookies and addon manifest settings
are never forwarded as request headers.

The HTTP leg from the egress host to the media CDN is **not encrypted**.
Operators should prefer HTTPS media providers where available. This is distinct
from encrypted Tailscale communication between YuFlix Core and an optional
egress host. Browser requests still use the installation's configured
HTTP/HTTPS origin.

## Default-direct routing and optional egress per addon

The normal installation places **Core and Web together on one host** and
connects to PostgreSQL 17 over the operator's own trusted network. It does
not require Tailscale, egress services or any other YuFlix installation.

Administrators can select the network route independently for each stream
addon under **Administration > Reprodução**:

- **Direto** (default): the YuFlix Core contacts the addon and media server
  itself, using DNS pinning, SSRF protection and bounded media streaming.
- **Egress privado**: source discovery, verification and media streaming use
  an operator-authorized private relay.
- **Automático**: try the direct discovery first; on a connection/provider
  failure, use an authorized egress. Normal media validation still applies.

The Egress and Automatic choices are shown only for addons whose host is
explicitly allowlisted in a configured egress. The network route is saved
in source_configs JSONB in PostgreSQL, and old records default to Direct.
Changing a route invalidates existing media tickets for that addon.

A relay's private endpoint, bearer secret and allowed hostnames are initial
bootstrap secrets in the Core environment. They are never returned by
public or authenticated APIs. A new installation without these bootstrap
settings uses Direct routing only. The relay service itself is optional
and deployed independently (see deploy/egress.compose.yaml).

Adding and rotating arbitrary relay credentials entirely through the panel
is future work. No private hostname, IP or credential is embedded in the
repository, container images or frontend.
