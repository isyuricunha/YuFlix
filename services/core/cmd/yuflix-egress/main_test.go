package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRelayAuthenticationAndHostAllowlist(t *testing.T) {
	h := &relay{token: strings.Repeat("x", 40), hosts: map[string]bool{"addon.example.com": true}, slots: make(chan struct{}, 3), cache: map[string]cacheEntry{}}
	req := func(path, token, payload string) *http.Request {
		r := httptest.NewRequest("POST", path, strings.NewReader(payload))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		return r
	}
	input := "{\"manifestUrl\":\"https://other.example.com/manifest.json\",\"type\":\"movie\",\"id\":\"tt123\"}"
	for _, tc := range []struct {
		token string
		want  int
	}{
		{"", 401}, {strings.Repeat("y", 40), 401}, {strings.Repeat("x", 40), 403},
	} {
		res := httptest.NewRecorder()
		h.serve(res, req("/v1/streams", tc.token, input))
		if res.Code != tc.want {
			t.Errorf("relay response %d, want %d", res.Code, tc.want)
		}
	}
}
