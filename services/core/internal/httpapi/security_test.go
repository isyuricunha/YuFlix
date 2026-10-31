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
	for _, origin := range []string{"", "http://stream.example.com", "https://stream.example.com/path", "https://stream.example.com?key=x"} {
		a := newAPI(Options{PublicOrigin: origin})
		if a.publicOrigin != "" {
			t.Fatalf("invalid origin was accepted: %q", origin)
		}
	}
	a := newAPI(Options{PublicOrigin: "https://stream.example.com"})
	if a.publicHost != "stream.example.com" {
		t.Fatal("HTTPS origin not parsed")
	}
}
