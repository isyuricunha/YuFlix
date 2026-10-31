package catalog

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestEgressConfigurationIsPrivateTailscaleOnly(t *testing.T) {
	token := strings.Repeat("s", 40)
	good, err := NewEgressTransport("http://100.99.20.3:62582", token, "addon.example.com")
	if err != nil || !good.handles("https://addon.example.com/config/manifest.json") {
		t.Fatalf("valid private egress rejected: %v", err)
	}
	if good.handles("https://other.example.com/manifest.json") {
		t.Fatal("non-allowlisted addon sent to egress")
	}
	for _, endpoint := range []string{
		"http://127.0.0.1:62582", "http://100.99.20.3:80",
		"http://100.99.20.3:62582/path", "https://100.99.20.3:62582",
		"http://public.example.com:62582",
	} {
		if _, err := NewEgressTransport(endpoint, token, "addon.example.com"); err == nil {
			t.Errorf("unsafe egress endpoint accepted")
		}
	}
	if _, err := NewEgressTransport("http://100.99.20.3:62582", "short", "addon.example.com"); err == nil {
		t.Fatal("weak token accepted")
	}
}
func TestEgressResponsesRequireValidatedMediaURLs(t *testing.T) {
	good, _ := NewEgressTransport("http://100.99.20.3:62582", strings.Repeat("s", 40), "addon.example.com")
	good.client = &http.Client{Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("missing egress auth")
		}
		return fakeJSON("{\"report\":{\"Streams\":[{\"Name\":\"video\",\"URL\":\"https://public.example.com/file.mp4\"}],\"Received\":1}}"), nil
	})}
	result, err := good.streams(context.Background(), "https://addon.example.com/manifest.json", "movie", "tt123")
	if err != nil || len(result.Streams) != 1 {
		t.Fatalf("valid egress response failed: %v", err)
	}
	good.client = &http.Client{Transport: fakeRoundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{\"report\":{\"Streams\":[{\"Name\":\"video\",\"URL\":\"http://localhost/private\"}]}}"))}, nil
	})}
	_, err = good.streams(context.Background(), "https://addon.example.com/manifest.json", "movie", "tt123")
	if err == nil {
		t.Fatal("unsafe remote media URL accepted")
	}
}
