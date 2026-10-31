package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/isyuricunha/YuFlix/services/core/internal/catalog"
)

type searchResult struct {
	SourceID string `json:"sourceId"`
	Catalog  string `json:"catalog"`
	catalog.Preview
}

func (a *api) searchCatalog(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if utf8.RuneCountInString(q) < 2 || utf8.RuneCountInString(q) > 64 || strings.ContainsAny(q, "\x00\r\n") {
		failure(w, 400, "a busca precisa ter de 2 a 64 caracteres")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := a.db.SearchCachedTitles(ctx, q)
	if err != nil {
		failure(w, 503, "busca temporariamente indisponível")
		return
	}
	out := make([]searchResult, 0, len(rows))
	for _, row := range rows {
		var preview catalog.Preview
		if json.Unmarshal(row.Item, &preview) != nil || preview.ID == "" || preview.Name == "" {
			continue
		}
		out = append(out, searchResult{SourceID: row.SourceID, Catalog: row.Catalog, Preview: preview})
	}
	writeJSON(w, 200, map[string]any{
		"items": out, "scope": "cached_catalogs", "query": q,
		"note": "A pesquisa usa metadados de catálogos já carregados ou armazenados.",
	})
}
