package httpapi

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/isyuricunha/YuFlix/services/core/internal/catalog"
	"github.com/isyuricunha/YuFlix/services/core/internal/db"
	"github.com/jackc/pgx/v5"
)

var sourceIDRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var catalogKeyRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,99}$`)

type catalogCacheEntry struct {
	items  []catalog.Preview
	expiry time.Time
}
type catalogCache struct {
	mu           sync.Mutex
	data         map[string]catalogCacheEntry
	refreshing   map[string]bool
	refreshSlots chan struct{}
}

func (a *api) listCatalogSources(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	sources, err := a.db.ListCatalogSources(ctx)
	if err != nil {
		failure(w, 503, "fontes indisponíveis")
		return
	}
	writeJSON(w, 200, map[string]any{"sources": sources})
}
func (a *api) addCatalogSource(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ManifestURL string `json:"manifestUrl"`
	}
	if err := decodeJSON(r, &input); err != nil {
		failure(w, 400, "dados inválidos")
		return
	}
	if _, err := catalog.ValidateManifestURL(input.ManifestURL); err != nil {
		failure(w, 400, "somente manifestos HTTPS públicos, sem tokens, estão disponíveis nesta etapa")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 13*time.Second)
	defer cancel()
	sources, err := a.db.ListCatalogSources(ctx)
	if err != nil {
		failure(w, 503, "fontes indisponíveis")
		return
	}
	if len(sources) >= 12 {
		failure(w, 409, "limite inicial de 12 fontes de catálogo")
		return
	}
	for _, s := range sources {
		if s.ManifestURL == input.ManifestURL {
			failure(w, 409, "fonte já cadastrada")
			return
		}
	}
	manifest, err := a.catalogClient.Manifest(ctx, input.ManifestURL)
	if err != nil {
		failure(w, 422, "manifesto inacessível ou sem catálogo suportado")
		return
	}
	cats := make([]db.StoredCatalog, 0, len(manifest.Catalogs))
	for _, c := range manifest.Catalogs {
		cats = append(cats, db.StoredCatalog{Type: c.Type, ID: c.ID, Name: c.Name})
	}
	src, err := a.db.AddCatalogSource(ctx, manifest.Name, input.ManifestURL, cats)
	if err != nil {
		failure(w, 409, "não foi possível cadastrar essa fonte")
		return
	}
	writeJSON(w, 201, map[string]any{"source": src})
}
func (a *api) deleteCatalogSource(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sourceID")
	if !sourceIDRE.MatchString(id) {
		failure(w, 400, "identificador inválido")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	ok, err := a.db.DeleteCatalogSource(ctx, id)
	if err != nil {
		failure(w, 503, "fontes indisponíveis")
		return
	}
	if !ok {
		failure(w, 404, "fonte não encontrada")
		return
	}
	a.cache.mu.Lock()
	for key := range a.cache.data {
		if len(key) >= len(id) && key[:len(id)] == id {
			delete(a.cache.data, key)
		}
	}
	a.cache.mu.Unlock()
	w.WriteHeader(204)
}
func (a *api) source(ctx context.Context, id string) (db.CatalogSource, error) {
	if !sourceIDRE.MatchString(id) {
		return db.CatalogSource{}, catalog.ErrInvalid
	}
	return a.db.GetCatalogSource(ctx, id)
}
func catalogAllowed(s db.CatalogSource, typ, id string) bool {
	for _, c := range s.Catalogs {
		if c.Type == typ && c.ID == id {
			return true
		}
	}
	return false
}
func (a *api) catalogItems(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sourceID")
	typ, catID := r.URL.Query().Get("type"), r.URL.Query().Get("catalog")
	if !catalogKeyRE.MatchString(typ) || !catalogKeyRE.MatchString(catID) {
		failure(w, 400, "catálogo inválido")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	src, err := a.source(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		failure(w, 404, "fonte não encontrada")
		return
	}
	if err != nil {
		failure(w, 400, "fonte inválida")
		return
	}
	if !src.Enabled || !catalogAllowed(src, typ, catID) {
		failure(w, 404, "catálogo não disponível")
		return
	}
	cacheKey := id + ":" + typ + ":" + catID
	a.cache.mu.Lock()
	cached, found := a.cache.data[cacheKey]
	a.cache.mu.Unlock()
	if src.CacheMode != "off" && found && time.Now().Before(cached.expiry) {
		writeJSON(w, 200, map[string]any{"items": cached.items, "sourceId": src.ID, "catalog": catID})
		return
	}
	if snapshot, age, ok := a.readSnapshot(ctx, id, typ, catID); src.CacheMode != "off" && ok && age < 48*time.Hour {
		a.cache.mu.Lock()
		a.cache.data[cacheKey] = catalogCacheEntry{items: snapshot, expiry: time.Now().Add(90 * time.Second)}
		a.cache.mu.Unlock()
		if age >= 6*time.Hour {
			a.refreshSnapshot(cacheKey, id, src.ManifestURL, typ, catID)
		}
		writeJSON(w, 200, map[string]any{"items": snapshot, "sourceId": src.ID, "catalog": catID})
		return
	}
	items, err := a.catalogClient.Catalog(ctx, src.ManifestURL, typ, catID)
	if err != nil {
		failure(w, 502, "provedor de catálogo temporariamente indisponível")
		return
	}
	if src.CacheMode != "off" {
		a.persistSnapshot(ctx, id, typ, catID, items)
	}
	a.cache.mu.Lock()
	if len(a.cache.data) > 150 {
		a.cache.data = map[string]catalogCacheEntry{}
	}
	a.cache.data[cacheKey] = catalogCacheEntry{items: items, expiry: time.Now().Add(90 * time.Second)}
	a.cache.mu.Unlock()
	writeJSON(w, 200, map[string]any{"items": items, "sourceId": src.ID, "catalog": catID})
}

func (a *api) catalogMeta(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sourceID")
	typ, metaID := r.URL.Query().Get("type"), r.URL.Query().Get("id")
	if !catalogKeyRE.MatchString(typ) || !catalog.ValidMediaID(metaID) {
		failure(w, 400, "título inválido")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	src, err := a.source(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		failure(w, 404, "fonte não encontrada")
		return
	}
	if err != nil {
		failure(w, 400, "fonte inválida")
		return
	}
	allowed := false
	for _, c := range src.Catalogs {
		if c.Type == typ {
			allowed = true
			break
		}
	}
	if !src.Enabled || !allowed {
		failure(w, 404, "metadados indisponíveis")
		return
	}
	info, err := a.catalogClient.Meta(ctx, src.ManifestURL, typ, metaID)
	if err != nil {
		failure(w, 502, "metadados temporariamente indisponíveis")
		return
	}
	writeJSON(w, 200, map[string]any{"meta": info, "sourceId": src.ID})
}
