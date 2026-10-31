package httpapi

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/isyuricunha/YuFlix/services/core/internal/db"
)

const cookieName = "__Host-yuflix_session"
const sessionDuration = 12 * time.Hour

type Options struct {
	Database     *db.DB
	PublicOrigin string
}
type api struct {
	db            *db.DB
	publicOrigin  string
	publicHost    string
	limit         *loginLimiter
	passwordSlots chan struct{}
}
type loginEntry struct {
	count int
	start time.Time
}
type loginLimiter struct {
	mu    sync.Mutex
	items map[string]loginEntry
}

func (l *loginLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.items) > 2048 {
		for k, e := range l.items {
			if now.Sub(e.start) > 15*time.Minute {
				delete(l.items, k)
			}
		}
		if len(l.items) > 2048 {
			return false
		}
	}
	e := l.items[key]
	if now.Sub(e.start) > 15*time.Minute {
		e = loginEntry{start: now}
	}
	e.count++
	l.items[key] = e
	return e.count <= 8
}
func (l *loginLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.items, key)
}
func newAPI(o Options) *api {
	a := &api{db: o.Database, limit: &loginLimiter{items: map[string]loginEntry{}}, passwordSlots: make(chan struct{}, 2)}
	v, err := url.Parse(o.PublicOrigin)
	if err == nil && v.Scheme == "https" && v.Host != "" && v.User == nil && v.Path == "" && v.RawQuery == "" && v.Fragment == "" {
		a.publicOrigin = strings.TrimSuffix(v.String(), "/")
		a.publicHost = v.Host
	}
	return a
}
func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
func failure(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": message})
}
func decodeJSON(r *http.Request, dst any) error {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return errors.New("JSON content type required")
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 8<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("unexpected trailing content")
	}
	return nil
}
func (a *api) requireSecure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.db == nil || a.publicOrigin == "" {
			failure(w, http.StatusServiceUnavailable, "autenticação aguardando configuração segura")
			return
		}
		// The reverse proxy is responsible for replacing client-supplied forwarded headers.
		// No direct network route may expose the Core outside its Docker network.
		if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
			failure(w, http.StatusForbidden, "HTTPS obrigatório")
			return
		}
		if !strings.EqualFold(r.Host, a.publicHost) {
			failure(w, http.StatusForbidden, "origem inválida")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			if r.Header.Get("Origin") != a.publicOrigin {
				failure(w, http.StatusForbidden, "origem inválida")
				return
			}
			switch r.Header.Get("Sec-Fetch-Site") {
			case "", "same-origin", "none":
			default:
				failure(w, http.StatusForbidden, "requisição entre origens bloqueada")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func cookieDigest(r *http.Request) ([]byte, error) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil, err
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil || len(raw) != 32 {
		return nil, errors.New("invalid session")
	}
	h := sha256.Sum256(raw)
	return h[:], nil
}
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: "", Path: "/", MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
}
func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: token, Path: "/", MaxAge: int(sessionDuration.Seconds()),
		Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
}
