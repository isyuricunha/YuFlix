# Catalog integrations and search

YuFlix contains a curated provider gallery under **Administration → Catalogs**.
Admins add a provider by name, without entering an addon URL. The backend resolves
reviewed public upstream manifests and validates the capabilities before saving
the source in PostgreSQL 17. Manifest endpoints are never exposed to viewers.

## Available catalog integrations

| Provider | Integration | Notes |
|---|---|---|
| The Movie Database | Ready | Movies, TV, trending and metadata through the public Stremio TMDb addon. Uses PT-BR data when offered upstream; this is not a native TMDb API integration. |
| Streaming Catalogs | Ready | Catalogs per streaming service, independent of the service's playback or a user subscription. Does not imply that a stream exists. |
| Anime Kitsu | Ready | Anime catalogs of popular, airing and trending series. Kitsu anime catalogs often return `series` entries, now accepted by the YuFlix catalog pipeline. Some stream addons do not support Kitsu IDs. |
| TMDB Collections | Ready | Preconfigured PT-BR collection browsing with popular, top-rated and new-release franchises. A collection contains film entries, not a playable movie by itself. |
| Anime Catalogs | Ready | Server-side preset with MyAnimeList top airing, AniList trending/seasonal and Kitsu top airing. The public default manifest is empty, so the provider is installed with verified non-secret configuration. |

**The Movie Database** is a *third-party TMDb-backed metadata addon*. A
separate direct integration using TMDb's official API and protected API token
is planned; the YuFlix UI will not request its secret in an unprotected field.
Collections use the existing Stremio provider interface, not the official TMDb
collection endpoint directly.

## Browser and search

- Dedicated category navigation for Movies, Series, Anime, Collections.
- Home distributes up to eight rails between enabled catalog sources, rather
  than always using the first four catalogs of the oldest providers.
- The Hero rotates real catalog entries and respects reduced-motion preferences.
- Search runs against PostgreSQL catalog snapshots, not provider URLs. This
  delivers a quick answer and survives reboots.
- Search is deliberately limited to previously fetched metadata (maximum 48
  results, snapshots newer than 48 hours); absence from search does not mean
  that an upstream provider lacks a title.
- Actual stream availability is checked only when Play is requested.

## Privacy and safety

No addon URL, token, or provider headers are delivered in browser source
summaries. Only administrators can install integrations. Custom manifest
installation is still supported in an advanced, collapsed panel.

Providers have a defined static list; arbitrary or misspelled provider IDs
return 404 and cannot trigger an external network request. Installation
validates the upstream manifest via the existing public-HTTPS pinned-DNS
client. Provider outages are reported rather than treated as successful.

## References

- https://docs.aiostreams.viren070.me/configuration/environment-variables/
- https://docs.aiostreams.viren070.me/apps/
- https://developer.themoviedb.org/reference/collection-details
- https://stremio.github.io/stremio-addon-sdk/api/responses/manifest.html
