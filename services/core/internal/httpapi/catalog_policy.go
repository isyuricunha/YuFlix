package httpapi

import (
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/isyuricunha/YuFlix/services/core/internal/db"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
	"time"
)

func validCatalogCacheMode(v string) bool { return v == "off" || v == "on-demand" || v == "precache" }
func (a *api) clearCatalogMemory(id string) {
	a.cache.mu.Lock()
	for key := range a.cache.data {
		if id == "" || strings.HasPrefix(key, id+":") {
			delete(a.cache.data, key)
		}
	}
	a.cache.mu.Unlock()
}
func (a *api) warmCatalogSource(source db.CatalogSource) {
	if !source.Enabled || source.CacheMode != "precache" {
		return
	}
	count := 0
	for _, cat := range source.Catalogs {
		if cat.Type != "movie" && cat.Type != "series" {
			continue
		}
		key := source.ID + ":" + cat.Type + ":" + cat.ID
		a.refreshSnapshot(key, source.ID, source.ManifestURL, cat.Type, cat.ID)
		count++
		if count == 2 {
			break
		}
	}
}
func (a *api) updateCatalogCacheMode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sourceID")
	if !sourceIDRE.MatchString(id) {
		failure(w, 400, "fonte inválida")
		return
	}
	var req struct {
		Mode string `json:"mode"`
	}
	if decodeJSON(r, &req) != nil || !validCatalogCacheMode(req.Mode) {
		failure(w, 400, "modo inválido")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	source, err := a.db.SetCatalogCacheMode(ctx, id, req.Mode)
	if errors.Is(err, pgx.ErrNoRows) {
		failure(w, 404, "fonte não encontrada")
		return
	}
	if err != nil {
		failure(w, 503, "não foi possível alterar o cache")
		return
	}
	a.clearCatalogMemory(id)
	a.warmCatalogSource(source)
	writeJSON(w, 200, map[string]any{"source": source})
}
func (a *api) updateAllCatalogCacheModes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode string `json:"mode"`
	}
	if decodeJSON(r, &req) != nil || !validCatalogCacheMode(req.Mode) {
		failure(w, 400, "modo inválido")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()
	if err := a.db.SetAllCatalogCacheModes(ctx, req.Mode); err != nil {
		failure(w, 503, "não foi possível salvar")
		return
	}
	sources, err := a.db.ListCatalogSources(ctx)
	if err != nil {
		failure(w, 503, "não foi possível carregar as fontes")
		return
	}
	a.clearCatalogMemory("")
	for _, s := range sources {
		a.warmCatalogSource(s)
	}
	writeJSON(w, 200, map[string]any{"sources": sources})
}
