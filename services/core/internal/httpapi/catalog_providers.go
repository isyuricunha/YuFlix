package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/isyuricunha/YuFlix/services/core/internal/db"
)

// Only administrator-selected, reviewed provider presets are accepted.
// No client-provided URL is resolved by the provider installation endpoint.
type catalogProvider struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Ready       bool   `json:"ready"`
	Note        string `json:"note,omitempty"`
	manifestURL string
}

var catalogProviders = []catalogProvider{
	{ID: "tmdb", Name: "The Movie Database", Description: "Filmes, séries, tendências e metadados.", Category: "Filmes e séries", Ready: true,
		manifestURL: "https://tmdb.elfhosted.com/manifest.json"},
	{ID: "streaming", Name: "Streaming Catalogs", Description: "Catálogos organizados por Netflix, HBO Max, Disney+, Prime Video e outras plataformas.", Category: "Plataformas", Ready: true,
		manifestURL: "https://7a82163c306e-stremio-netflix-catalog-addon.baby-beamup.club/manifest.json"},
	{ID: "anime-kitsu", Name: "Anime Kitsu", Description: "Animes em alta, em exibição e populares.", Category: "Animes", Ready: true,
		manifestURL: "https://anime-kitsu.strem.fun/manifest.json"},
	{ID: "anime-catalogs", Name: "Anime Catalogs", Description: "Catálogos MyAnimeList, AniDB, AniList e Kitsu.", Category: "Animes",
		Note: "O provedor exige configuração antes de expor catálogos. Integração nativa em preparação."},
	{ID: "tmdb-collections", Name: "TMDB Collections", Description: "Franquias de filmes e coleções cinematográficas.", Category: "Coleções", Ready: true,
		manifestURL: "https://61ab9c85a149-tmdb-collections.baby-beamup.club/" + url.PathEscape(`{"language":"pt-BR","catalogList":["popular","topRated","newReleases"]}`) + "/manifest.json"},
}

func (a *api) listCatalogProviders(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	sources, err := a.db.ListCatalogSources(ctx)
	if err != nil {
		failure(w, 503, "catálogos indisponíveis")
		return
	}
	type display struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Ready       bool   `json:"ready"`
		Installed   bool   `json:"installed"`
		Note        string `json:"note,omitempty"`
	}
	out := make([]display, 0, len(catalogProviders))
	for _, p := range catalogProviders {
		installed := false
		if p.Ready {
			for _, s := range sources {
				if s.ManifestURL == p.manifestURL {
					installed = true
					break
				}
			}
		}
		out = append(out, display{p.ID, p.Name, p.Description, p.Category, p.Ready, installed, p.Note})
	}
	writeJSON(w, 200, map[string]any{"providers": out})
}

func (a *api) installCatalogProvider(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "providerID")
	var provider catalogProvider
	for _, p := range catalogProviders {
		if p.ID == id {
			provider = p
			break
		}
	}
	if provider.ID == "" {
		failure(w, 404, "integração desconhecida")
		return
	}
	if !provider.Ready {
		failure(w, 409, "integração ainda exige suporte adicional")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 14*time.Second)
	defer cancel()
	sources, err := a.db.ListCatalogSources(ctx)
	if err != nil {
		failure(w, 503, "catálogos indisponíveis")
		return
	}
	if len(sources) >= 12 {
		failure(w, 409, "limite inicial de 12 provedores de catálogo")
		return
	}
	for _, s := range sources {
		if s.ManifestURL == provider.manifestURL {
			failure(w, 409, "integração já instalada")
			return
		}
	}
	manifest, err := a.catalogClient.Manifest(ctx, provider.manifestURL)
	if err != nil {
		failure(w, 422, "catálogo do provedor indisponível ou incompatível")
		return
	}
	cats := make([]db.StoredCatalog, 0, len(manifest.Catalogs))
	for _, c := range manifest.Catalogs {
		cats = append(cats, db.StoredCatalog{Type: c.Type, ID: c.ID, Name: c.Name})
	}
	if len(cats) == 0 {
		failure(w, 422, "nenhum catálogo compatível")
		return
	}
	// Preserve original addon names and keep the upstream URL in the database.
	// Client receives a safe source summary only.
	src, err := a.db.AddCatalogSource(ctx, provider.Name, provider.manifestURL, cats)
	if err != nil {
		failure(w, 409, "não foi possível instalar este catálogo")
		return
	}
	writeJSON(w, 201, map[string]any{"source": src})
}
