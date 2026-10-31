package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/isyuricunha/YuFlix/services/core/internal/db"
	"github.com/isyuricunha/YuFlix/services/core/internal/media"
	"github.com/jackc/pgx/v5"
)

const mediaCacheSettingKey = "media.cache_policy"

func (a *api) currentMediaCachePolicy(ctx context.Context) (media.CachePolicy, error) {
	var raw []byte
	var result media.CachePolicy
	// Only one setting is accessed: never expose unrelated keys or secrets.
	values, err := a.db.GetSettings(ctx)
	if err != nil {
		return result, err
	}
	raw = values[mediaCacheSettingKey]
	if len(raw) == 0 {
		return media.DefaultCachePolicy(), nil
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return result, err
	}
	if err := result.Validate(); err != nil {
		return result, err
	}
	return result, nil
}

func (a *api) getMediaCachePolicy(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	policy, err := a.currentMediaCachePolicy(ctx)
	if err != nil {
		failure(w, 503, "configuração de mídia indisponível")
		return
	}
	writeJSON(w, 200, map[string]any{
		"policy": policy, "defaults": media.DefaultCachePolicy(),
		"active": false,
		"status": "O worker HLS ainda não está habilitado nesta instalação.",
	})
}

func (a *api) putMediaCachePolicy(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Policy media.CachePolicy `json:"policy"`
	}
	if err := decodeJSON(r, &input); err != nil || input.Policy.Validate() != nil {
		failure(w, 400, "valores de cache inválidos")
		return
	}
	user := r.Context().Value(contextUserKey{}).(db.User)
	data, err := json.Marshal(input.Policy)
	if err != nil {
		failure(w, 400, "configuração inválida")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err = a.db.SetSetting(ctx, mediaCacheSettingKey, data, user.ID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			failure(w, 404, "configuração não encontrada")
			return
		}
		failure(w, 503, "não foi possível salvar o cache")
		return
	}
	writeJSON(w, 200, map[string]any{
		"policy": input.Policy, "defaults": media.DefaultCachePolicy(),
		"active": false, "status": "Configuração salva. O worker HLS ainda não está habilitado.",
	})
}
