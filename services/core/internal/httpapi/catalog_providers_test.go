package httpapi

import (
	"testing"

	"github.com/isyuricunha/YuFlix/services/core/internal/catalog"
)

func TestCatalogProviderPresetsAreReviewedAndSafe(t *testing.T) {
	seen := map[string]bool{}
	ready := 0
	for _, provider := range catalogProviders {
		if seen[provider.ID] || provider.ID == "" {
			t.Fatalf("duplicate provider ID %q", provider.ID)
		}
		seen[provider.ID] = true
		if provider.Ready {
			ready++
			if _, err := catalog.ValidateManifestURL(provider.manifestURL); err != nil {
				t.Fatalf("invalid provider preset %q: %v", provider.ID, err)
			}
		} else if provider.manifestURL != "" {
			t.Fatalf("unavailable provider URL must not be exposed: %s", provider.ID)
		}
	}
	if ready != 5 || !seen["anime-catalogs"] || !seen["tmdb-collections"] {
		t.Fatalf("expected reviewed metadata, streaming, anime, collections providers")
	}
}
