package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicRoutes(t *testing.T) {
	router := NewRouter()
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/healthz", 200},
		{"/readyz", 200},
		{"/api/v1/system/info", 200},
		{"/api/v1/admin/status", 503},
		{"/api/v1/playback/start", 404},
	} {
		t.Run(tc.path, func(t *testing.T) {
			r := httptest.NewRecorder()
			router.ServeHTTP(r, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if r.Code != tc.want {
				t.Fatalf("got %d, want %d", r.Code, tc.want)
			}
			if got := r.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("missing security header: %q", got)
			}
			if got := r.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Errorf("unexpected CORS allowance: %s", got)
			}
		})
	}
}
func TestPublicInfoDoesNotPretendPlayback(t *testing.T) {
	resp := httptest.NewRecorder()
	NewRouter().ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/system/info", nil))
	var got map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["playbackReady"] != false || got["phase"] != "foundation" {
		t.Fatalf("unexpected state: %v", got)
	}
	if resp.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("missing cache protection")
	}
}
