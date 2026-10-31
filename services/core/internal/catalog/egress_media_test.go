package catalog

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCoreEgressRangeAndProbe(t *testing.T) {
	e, err := NewEgressTransport("http://100.99.20.3:62582", strings.Repeat("s", 40), "addon.example.com")
	if err != nil {
		t.Fatal(err)
	}
	e.client.Transport = fakeRoundTrip(func(req *http.Request) (*http.Response, error) {
		if !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ") {
			t.Fatal("missing private bearer")
		}
		if req.URL.Path != "/v1/probe" {
			t.Fatal("probe endpoint wrong")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})
	c := &Client{egress: e}
	if err := c.ProbeViaEgress(context.Background(), "https://addon.example.com/manifest.json", "movie", "tt123", "https://media.example.com/x.mp4"); err != nil {
		t.Fatal(err)
	}
	e.client.Transport = fakeRoundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/media" || req.Header.Get("Range") != "bytes=0-7" {
			t.Fatal("relay routing or range failed")
		}
		return &http.Response{StatusCode: 206, Header: http.Header{
			"Content-Type": []string{"video/mp4"}, "Content-Range": []string{"bytes 0-7/100"},
			"Set-Cookie": []string{"forbidden=secret"}}, Body: io.NopCloser(bytes.NewReader([]byte("12345678")))}, nil
	})
	req := httptest.NewRequest("GET", "/api/v1/media/play/opaque", nil)
	req.Header.Set("Range", "bytes=0-7")
	rec := httptest.NewRecorder()
	c.RelayViaEgress(rec, req, "https://media.example.com/x.mp4")
	if rec.Code != 206 || rec.Body.String() != "12345678" || rec.Header().Get("Set-Cookie") != "" {
		t.Fatalf("Core proxy leak or Range failure %d", rec.Code)
	}
}
