package main

import (
	"github.com/isyuricunha/YuFlix/services/core/internal/media"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
func TestEgressMediaNotOpenProxy(t *testing.T) {
	u := "https://media.example.com/clip.mp4"
	h := &relay{token: strings.Repeat("s", 40), hosts: map[string]bool{"addon.example.com": true},
		allowed: map[string]time.Time{}, videoSlots: make(chan struct{}, 1)}
	body := "{\"url\":\"" + u + "\"}"
	request := func() *http.Request {
		r := httptest.NewRequest("POST", "/v1/media", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+h.token)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Range", "bytes=0-7")
		return r
	}
	out := httptest.NewRecorder()
	h.serve(out, request())
	if out.Code != 404 {
		t.Fatalf("undiscovered media URL accepted %d", out.Code)
	}
	h.allowed[u] = time.Now().Add(time.Minute)
	h.direct = &media.Direct{HTTP: &http.Client{Transport: testTransport(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Cookie") != "" || req.Header.Get("Authorization") != "" {
			t.Fatal("internal auth forwarded upstream")
		}
		if req.Header.Get("Range") != "bytes=0-7" {
			t.Fatal("Range not preserved")
		}
		return &http.Response{StatusCode: 206, Header: http.Header{
			"Content-Type": []string{"video/mp4"}, "Content-Range": []string{"bytes 0-7/100"},
			"Set-Cookie": []string{"private=secret"}}, Body: io.NopCloser(strings.NewReader("12345678"))}, nil
	})}}
	out = httptest.NewRecorder()
	h.serve(out, request())
	if out.Code != 206 || out.Body.String() != "12345678" || out.Header().Get("Set-Cookie") != "" {
		t.Fatalf("incorrect or unsafe media relay result %d", out.Code)
	}
}
