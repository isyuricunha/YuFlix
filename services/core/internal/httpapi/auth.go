package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"

	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/isyuricunha/YuFlix/services/core/internal/auth"
	"github.com/isyuricunha/YuFlix/services/core/internal/db"
	"github.com/jackc/pgx/v5"
)

type contextUserKey struct{}

func (a *api) withUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		digest, err := cookieDigest(r)
		if err != nil {
			failure(w, 401, "sessão necessária")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		u, err := a.db.UserBySession(ctx, digest, transportFromContext(r))
		if errors.Is(err, pgx.ErrNoRows) {
			failure(w, 401, "sessão inválida")
			return
		}
		if err != nil {
			failure(w, 503, "banco indisponível")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextUserKey{}, u)))
	})
}
func (a *api) adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := r.Context().Value(contextUserKey{}).(db.User)
		if u.Role != "admin" {
			failure(w, 403, "permissão administrativa necessária")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (a *api) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		failure(w, 400, "JSON inválido")
		return
	}
	username := strings.ToLower(strings.TrimSpace(input.Username))
	if !auth.ValidUsername(username) || len(input.Password) > 128 || len(input.Password) < 1 {
		failure(w, 401, "usuário ou senha inválidos")
		return
	}
	// Bound failed attempts per username. Forwarded client IP headers are untrusted.
	key := username
	if !a.limit.allow(key) {
		failure(w, 429, "muitas tentativas; tente mais tarde")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	u, stored, err := a.db.UserForLogin(ctx, username)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		failure(w, 503, "banco indisponível")
		return
	}
	// Bound concurrent Argon2 work to protect a memory-limited Core.
	select {
	case a.passwordSlots <- struct{}{}:
		defer func() { <-a.passwordSlots }()
	case <-ctx.Done():
		failure(w, 503, "limite temporário de autenticação")
		return
	}
	// Unknown and disabled accounts perform comparable password verification.
	if err != nil || !u.Enabled {
		stored, _ = auth.HashPassword("constant-time placeholder password")
	}
	verified := auth.VerifyPassword(input.Password, stored) && err == nil && u.Enabled
	if !verified {
		failure(w, 401, "usuário ou senha inválidos")
		return
	}
	token, raw, err := auth.Token()
	if err != nil {
		failure(w, 500, "não foi possível iniciar sessão")
		return
	}
	digest := sha256.Sum256(raw)
	if err := a.db.CreateSession(ctx, u.ID, digest[:], time.Now().Add(sessionDuration), transportFromContext(r)); err != nil {
		failure(w, 503, "banco indisponível")
		return
	}
	a.limit.reset(key)
	setSessionCookie(w, token, transportFromContext(r))
	writeJSON(w, 200, map[string]any{"user": u})
}
func (a *api) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"user": r.Context().Value(contextUserKey{}).(db.User)})
}
func (a *api) logout(w http.ResponseWriter, r *http.Request) {
	digest, err := cookieDigest(r)
	if err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := a.db.RevokeSession(ctx, digest); err != nil {
			failure(w, 503, "banco indisponível")
			return
		}
	}
	clearSessionCookie(w, transportFromContext(r))
	w.WriteHeader(204)
}
func (a *api) listUsers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	users, err := a.db.ListUsers(ctx)
	if err != nil {
		failure(w, 503, "banco indisponível")
		return
	}
	writeJSON(w, 200, map[string]any{"users": users})
}
func (a *api) createUser(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := decodeJSON(r, &input); err != nil {
		failure(w, 400, "JSON inválido")
		return
	}
	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	if !auth.ValidUsername(input.Username) || (input.Role != "admin" && input.Role != "member") {
		failure(w, 400, "usuário ou função inválidos")
		return
	}
	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		failure(w, 400, "senha deve ter entre 12 e 128 bytes")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()
	u, err := a.db.AddUser(ctx, input.Username, hash, input.Role)
	if err != nil {
		failure(w, 409, "não foi possível cadastrar o usuário")
		return
	}
	writeJSON(w, 201, map[string]any{"user": u})
}

var allowedSettings = map[string]func(json.RawMessage) bool{
	"catalog.default_language": func(v json.RawMessage) bool {
		var x string
		return json.Unmarshal(v, &x) == nil && (x == "pt-BR" || x == "en-US")
	},
	"playback.prefer_direct": func(v json.RawMessage) bool {
		var x bool
		return json.Unmarshal(v, &x) == nil && (string(v) == "true" || string(v) == "false")
	},
}

func (a *api) getSettings(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	s, err := a.db.GetSettings(ctx)
	if err != nil {
		failure(w, 503, "banco indisponível")
		return
	}
	for k := range s {
		if _, ok := allowedSettings[k]; !ok {
			delete(s, k)
		}
	}
	out := map[string]json.RawMessage{}
	for k, v := range s {
		out[k] = json.RawMessage(v)
	}
	writeJSON(w, 200, map[string]any{"settings": out, "effective": false})
}
func (a *api) putSetting(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	validate, ok := allowedSettings[key]
	if !ok {
		failure(w, 404, "configuração desconhecida")
		return
	}
	var input struct {
		Value json.RawMessage `json:"value"`
	}
	if err := decodeJSON(r, &input); err != nil || !validate(input.Value) {
		failure(w, 400, "valor inválido")
		return
	}
	u := r.Context().Value(contextUserKey{}).(db.User)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := a.db.SetSetting(ctx, key, input.Value, u.ID); err != nil {
		failure(w, 503, "banco indisponível")
		return
	}
	writeJSON(w, 200, map[string]any{"key": key, "value": input.Value, "effective": false})
}
