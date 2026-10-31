package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func (a *api) updateStreamAddonPolicy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sourceID")
	if !sourceIDRE.MatchString(id) {
		failure(w, 400, "identificador inválido")
		return
	}
	var input struct {
		Priority  int `json:"priority"`
		TimeoutMs int `json:"timeoutMs"`
	}
	if decodeJSON(r, &input) != nil || input.Priority < 1 || input.Priority > 1000 || input.TimeoutMs < 1000 || input.TimeoutMs > 10000 {
		failure(w, 400, "prioridade ou timeout inválido")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	updated, err := a.db.UpdateStreamAddonPolicy(ctx, id, input.Priority, input.TimeoutMs)
	if errors.Is(err, pgx.ErrNoRows) {
		failure(w, 404, "provedor não encontrado")
		return
	}
	if err != nil {
		failure(w, 503, "não foi possível salvar a configuração")
		return
	}
	writeJSON(w, 200, map[string]any{"source": updated})
}
