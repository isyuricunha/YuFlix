package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrivateEndpointsFailClosedWithoutDatabase(t *testing.T) {
	router := NewRouter(Options{PublicOrigin: "https://stream.example.com"})
	for _, route := range []struct{ method, path string }{
		{"POST", "/api/v1/auth/login"},
		{"GET", "/api/v1/auth/me"},
		{"POST", "/api/v1/auth/logout"},
		{"GET", "/api/v1/admin/users"},
		{"POST", "/api/v1/admin/users"},
		{"GET", "/api/v1/admin/settings"},
		{"PUT", "/api/v1/admin/settings/catalog.default_language"},
	} {
		t.Run(route.method+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, strings.NewReader("{}"))
			req.Host = "stream.example.com"
			req.Header.Set("Origin", "https://stream.example.com")
			req.Header.Set("X-Forwarded-Proto", "https")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("got %d want 503", rec.Code)
			}
			if rec.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("CORS unexpectedly enabled")
			}
		})
	}
}
func TestOriginConfiguration(t *testing.T) {
	for _, origin := range []string{"", "http://stream.example.com", "https://stream.example.com/path", "https://stream.example.com?key=x", "https://bad@stream.example.com"} {
		a := newAPI(Options{PublicOrigin: origin})
		if len(a.origins) != 0 {
			t.Fatalf("invalid origin accepted: %q", origin)
		}
	}
	a := newAPI(Options{
		PublicOrigin:      "https://stream.example.com",
		AllowedOrigins:    []string{"http://stream.example.com:62471"},
		AllowInsecureHTTP: true, TrustProxy: true,
	})
	if len(a.origins) != 2 {
		t.Fatalf("expected both origins, got %d", len(a.origins))
	}
	insecure := newAPI(Options{AllowedOrigins: []string{"http://stream.example.com:62471"}})
	if len(insecure.origins) != 0 {
		t.Fatal("insecure origin enabled without explicit opt-in")
	}
}

func TestTransportCookieIsolation(t *testing.T) {
	for _, test := range []struct {
		scheme, name string
		secure       bool
	}{
		{"http", httpCookieName, false}, {"https", cookieName, true},
	} {
		t.Run(test.scheme, func(t *testing.T) {
			r := httptest.NewRecorder()
			setSessionCookie(r, "example", test.scheme)
			cookies := r.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("cookies: %v", cookies)
			}
			c := cookies[0]
			if c.Name != test.name || c.Secure != test.secure ||
				!c.HttpOnly || c.Path != "/" || c.SameSite != http.SameSiteStrictMode {
				t.Fatalf("invalid cookie: %+v", c)
			}
			x := httptest.NewRecorder()
			clearSessionCookie(x, test.scheme)
			cleared := x.Result().Cookies()
			if len(cleared) != 1 || cleared[0].Name != test.name || cleared[0].MaxAge != -1 {
				t.Fatalf("cookie not cleared: %+v", cleared)
			}
		})
	}
}
func TestForwardedProtoOnlyWhenTrusted(t *testing.T) {
	req := httptest.NewRequest("GET", "http://stream.example.com/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	if got := newAPI(Options{}).requestTransport(req); got != "http" {
		t.Fatal("untrusted header accepted", got)
	}
	if got := newAPI(Options{TrustProxy: true}).requestTransport(req); got != "https" {
		t.Fatal("trusted proxy ignored", got)
	}
	req.Header.Set("X-Forwarded-Proto", "https, http")
	if got := newAPI(Options{TrustProxy: true}).requestTransport(req); got != "" {
		t.Fatal("ambiguous forwarded proto accepted", got)
	}
}
