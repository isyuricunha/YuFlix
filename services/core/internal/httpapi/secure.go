package httpapi

import (
	"context"
	"crypto/rand"
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

	"github.com/isyuricunha/YuFlix/services/core/internal/catalog"
	"github.com/isyuricunha/YuFlix/services/core/internal/db"
	"github.com/isyuricunha/YuFlix/services/core/internal/media"
)

const cookieName = "__Host-yuflix_session"
const httpCookieName = "yuflix_http_session"
const sessionDuration = 12 * time.Hour

type Options struct {
	Database          *db.DB
	PublicOrigin      string // Legacy single HTTPS origin.
	AllowedOrigins    []string
	AllowInsecureHTTP bool
	TrustProxy        bool // Only when Core is isolated behind a reverse proxy that overwrites X-Forwarded-Proto.
}
type api struct {
	db            *db.DB
	origins       map[string]struct{}
	trustProxy    bool
	limit         *loginLimiter
	passwordSlots chan struct{}
	catalogClient *catalog.Client
	cache         *catalogCache
	direct        *media.Direct
	video         *videoTickets
	mediaSlots    chan struct{}
	relaySlots    chan struct{}
	streamKey     [32]byte
}
type transportKey struct{}

func transportFromContext(r *http.Request) string {
	v, _ := r.Context().Value(transportKey{}).(string)
	return v
}
func (a *api) requestTransport(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if !a.trustProxy {
		return "http"
	}
	// An isolated trusted reverse proxy must overwrite this header.
	switch r.Header.Get("X-Forwarded-Proto") {
	case "https":
		return "https"
	case "http":
		return "http"
	default:
		return ""
	}
}
func validOrigin(input string, allowHTTP bool) (string, bool) {
	if input == "" || strings.TrimSpace(input) != input {
		return "", false
	}
	u, err := url.Parse(input)
	if err != nil || u.Opaque != "" || u.Host == "" || u.Hostname() == "" || u.User != nil ||
		u.Path != "" || u.RawPath != "" || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery ||
		(u.Scheme != "https" && !(allowHTTP && u.Scheme == "http")) {
		return "", false
	}
	if u.String() != input {
		return "", false
	}
	return input, true
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
func (l *loginLimiter) reset(key string) { l.mu.Lock(); defer l.mu.Unlock(); delete(l.items, key) }
func newAPI(o Options) *api {
	a := &api{db: o.Database, origins: map[string]struct{}{}, trustProxy: o.TrustProxy,
		limit: &loginLimiter{items: map[string]loginEntry{}}, passwordSlots: make(chan struct{}, 2), catalogClient: catalog.NewClient(), cache: &catalogCache{data: map[string]catalogCacheEntry{}}, direct: media.NewDirect(), video: &videoTickets{items: map[string]videoTicket{}}, mediaSlots: make(chan struct{}, 4), relaySlots: make(chan struct{}, 8)}
	if _, err := rand.Read(a.streamKey[:]); err != nil {
		panic("secure media selection unavailable")
	}
	for _, origin := range append([]string{o.PublicOrigin}, o.AllowedOrigins...) {
		if v, ok := validOrigin(origin, o.AllowInsecureHTTP); ok {
			a.origins[v] = struct{}{}
		}
	}
	return a
}
func (a *api) allowedRequest(r *http.Request) (string, string, bool) {
	scheme := a.requestTransport(r)
	if scheme == "" {
		return "", "", false
	}
	origin := scheme + "://" + r.Host
	_, ok := a.origins[origin]
	return scheme, origin, ok
}
func (a *api) authenticationReady(r *http.Request) bool {
	if a.db == nil {
		return false
	}
	_, _, ok := a.allowedRequest(r)
	return ok
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
	var more any
	if err := dec.Decode(&more); !errors.Is(err, io.EOF) {
		return errors.New("unexpected trailing content")
	}
	return nil
}
func (a *api) requireTransport(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.db == nil {
			failure(w, http.StatusServiceUnavailable, "autenticação aguardando PostgreSQL")
			return
		}
		scheme, origin, ok := a.allowedRequest(r)
		if !ok {
			failure(w, http.StatusForbidden, "origem ou protocolo não autorizado")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			if r.Header.Get("Origin") != origin {
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
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), transportKey{}, scheme)))
	})
}
func sessionCookieName(scheme string) string {
	switch scheme {
	case "https":
		return cookieName
	case "http":
		return httpCookieName
	default:
		return ""
	}
}
func cookieDigest(r *http.Request) ([]byte, error) {
	name := sessionCookieName(transportFromContext(r))
	if name == "" {
		return nil, errors.New("unknown session transport")
	}
	cookie, err := r.Cookie(name)
	if err != nil {
		return nil, err
	}
	raw, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil || len(raw) != 32 {
		return nil, errors.New("invalid session")
	}
	h := sha256.Sum256(raw)
	return h[:], nil
}
func clearSessionCookie(w http.ResponseWriter, scheme string) {
	name := sessionCookieName(scheme)
	if name == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1,
		Secure: scheme == "https", HttpOnly: true, SameSite: http.SameSiteStrictMode})
}
func setSessionCookie(w http.ResponseWriter, token, scheme string) {
	name := sessionCookieName(scheme)
	if name == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{Name: name, Value: token, Path: "/", MaxAge: int(sessionDuration.Seconds()),
		Secure: scheme == "https", HttpOnly: true, SameSite: http.SameSiteStrictMode})
}
