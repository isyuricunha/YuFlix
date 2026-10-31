package httpapi

import (
	"context"
	"errors"
	"regexp"

	"github.com/isyuricunha/YuFlix/services/core/internal/catalog"
)

// This public metadata provider is not a playback source. It is used only
// to recover details for IMDb-identified titles from catalog-only addons,
// never to probe or relay video, and never for provider-defined opaque IDs.
const publicCinemetaManifest = "https://v3-cinemeta.strem.io/manifest.json"

var imdbTitleID = regexp.MustCompile(`^tt[0-9]{7,10}$`)

type metadataReader func(context.Context, string, string, string) (catalog.Detail, error)

func resolveCatalogMetadata(ctx context.Context, providerURL, typ, id string, read metadataReader) (catalog.Detail, error) {
	// Streaming Catalogs is a catalog-only addon. For IMDb IDs it has no
	// metadata endpoint: skip the guaranteed-to-fail request, avoiding a
	// needless timeout before the viewer can open the title page.
	if imdbTitleID.MatchString(id) && (typ == "movie" || typ == "series") {
		for _, preset := range catalogProviders {
			if preset.ID == "streaming" && preset.manifestURL == providerURL {
				meta, err := read(ctx, publicCinemetaManifest, typ, id)
				if err == nil && meta.ID == id && meta.Type == typ {
					return meta, nil
				}
				if err != nil {
					return catalog.Detail{}, err
				}
				return catalog.Detail{}, catalog.ErrInvalid
			}
		}
	}
	meta, err := read(ctx, providerURL, typ, id)
	if err == nil {
		return meta, nil
	}
	// Only identical IMDb media IDs can be mapped without guesswork. For
	// unsupported IDs, surface an error rather than showing another title.
	if !imdbTitleID.MatchString(id) || (typ != "movie" && typ != "series") ||
		providerURL == publicCinemetaManifest || ctx.Err() != nil {
		return catalog.Detail{}, err
	}
	fallback, fallbackErr := read(ctx, publicCinemetaManifest, typ, id)
	if fallbackErr == nil && fallback.ID == id && fallback.Type == typ {
		return fallback, nil
	}
	if ctx.Err() != nil {
		return catalog.Detail{}, ctx.Err()
	}
	// Keep upstream errors private and avoid returning inaccurate metadata.
	return catalog.Detail{}, errors.Join(err, fallbackErr, catalog.ErrInvalid)
}
