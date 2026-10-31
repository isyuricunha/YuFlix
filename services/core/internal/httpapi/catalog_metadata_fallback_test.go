package httpapi

import (
	"context"
	"errors"
	"testing"

	"github.com/isyuricunha/YuFlix/services/core/internal/catalog"
)

func TestCatalogMetadataUsesPrimaryWhenHealthy(t *testing.T) {
	var calls []string
	reader := func(_ context.Context, url, typ, id string) (catalog.Detail, error) {
		calls = append(calls, url)
		return catalog.Detail{Preview: catalog.Preview{ID: id, Type: typ, Name: "Title"}}, nil
	}
	meta, err := resolveCatalogMetadata(context.Background(), "https://catalog.example.com/manifest.json", "movie", "tt1234567", reader)
	if err != nil || meta.ID != "tt1234567" || len(calls) != 1 {
		t.Fatalf("unexpected metadata fallback: %+v (%v), calls %d", meta, err, len(calls))
	}
}

func TestCatalogMetadataRecoversCatalogOnlyIMDbTitles(t *testing.T) {
	for _, typ := range []string{"movie", "series"} {
		t.Run(typ, func(t *testing.T) {
			var calls []string
			reader := func(_ context.Context, provider, kind, id string) (catalog.Detail, error) {
				calls = append(calls, provider)
				if provider != publicCinemetaManifest {
					return catalog.Detail{}, catalog.ErrInvalid
				}
				return catalog.Detail{Preview: catalog.Preview{ID: id, Type: kind, Name: "Recovered"}}, nil
			}
			meta, err := resolveCatalogMetadata(context.Background(),
				"https://catalog.example.com/manifest.json", typ, "tt12345678", reader)
			if err != nil || meta.Name != "Recovered" || len(calls) != 2 ||
				calls[1] != publicCinemetaManifest {
				t.Fatalf("expected safe metadata recovery: %+v (%v) calls %+v", meta, err, calls)
			}
		})
	}
}

func TestCatalogMetadataNeverGuessesOpaqueIDs(t *testing.T) {
	for _, test := range []struct{ typ, id string }{
		{"movie", "tmdb:155"}, {"series", "kitsu:7442"}, {"anime", "tt12345678"},
		{"collections", "tt12345678"}, {"movie", "tt1234"}, {"series", "../tt1234567"},
	} {
		count := 0
		_, err := resolveCatalogMetadata(context.Background(), "https://catalog.example.com/manifest.json",
			test.typ, test.id, func(_ context.Context, _, _, _ string) (catalog.Detail, error) {
				count++
				return catalog.Detail{}, catalog.ErrInvalid
			})
		if count != 1 || !errors.Is(err, catalog.ErrInvalid) {
			t.Fatalf("unsafe fallback for %s/%s: %v calls=%d", test.typ, test.id, err, count)
		}
	}
}

func TestCatalogMetadataRejectsMismatchedFallback(t *testing.T) {
	count := 0
	_, err := resolveCatalogMetadata(context.Background(), "https://catalog.example.com/manifest.json",
		"movie", "tt1234567", func(_ context.Context, _, _, _ string) (catalog.Detail, error) {
			count++
			if count == 1 {
				return catalog.Detail{}, catalog.ErrInvalid
			}
			return catalog.Detail{Preview: catalog.Preview{ID: "tt7654321", Type: "movie", Name: "Wrong"}}, nil
		})
	if count != 2 || err == nil {
		t.Fatalf("incompatible fallback accepted: %v calls=%d", err, count)
	}
}

func TestCatalogMetadataNeverRetriesCinemetaOrCancelledRequests(t *testing.T) {
	count := 0
	reader := func(_ context.Context, _, _, _ string) (catalog.Detail, error) {
		count++
		return catalog.Detail{}, catalog.ErrInvalid
	}
	_, _ = resolveCatalogMetadata(context.Background(), publicCinemetaManifest, "movie", "tt1234567", reader)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = resolveCatalogMetadata(ctx, "https://catalog.example.com/manifest.json", "movie", "tt1234567", reader)
	if count != 2 {
		t.Fatalf("unexpected retries after cancellation or Cinemeta: %d", count)
	}
}

// Catalog-only providers should not delay the detail page with a failing
// /meta request before consulting the supported metadata provider.
func TestStreamingCatalogIMDbDetailsUseFastPath(t *testing.T) {
	var upstream string
	for _, provider := range catalogProviders {
		if provider.ID == "streaming" {
			upstream = provider.manifestURL
			break
		}
	}
	if upstream == "" {
		t.Fatal("streaming catalog preset missing")
	}
	for _, typ := range []string{"movie", "series"} {
		var calls []string
		meta, err := resolveCatalogMetadata(context.Background(), upstream, typ, "tt1234567",
			func(_ context.Context, provider, kind, id string) (catalog.Detail, error) {
				calls = append(calls, provider)
				return catalog.Detail{Preview: catalog.Preview{ID: id, Type: kind, Name: "Recovered"}}, nil
			})
		if err != nil || meta.Name != "Recovered" || len(calls) != 1 || calls[0] != publicCinemetaManifest {
			t.Fatalf("streaming title metadata failed for %s (%v): %+v", typ, err, calls)
		}
	}
}
