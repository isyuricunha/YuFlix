package catalog

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeRoundTrip func(*http.Request) (*http.Response, error)

func (f fakeRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
func fakeJSON(body string) *http.Response {
	return &http.Response{
		StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(body)),
	}
}
func TestStreamSourceManifestAndSafeCandidates(t *testing.T) {
	endpoint := "https://addon.example.com/manifest.json"
	client := &Client{client: &http.Client{Transport: fakeRoundTrip(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/manifest.json":
			return fakeJSON(`{"name":"Sample Source","id":"sample","resources":[{"name":"stream","types":["movie"]}]}`), nil
		case "/stream/movie/tt123.json":
			return fakeJSON(`{"streams":[
    {"name":"Clear MP4","url":"https://video.example.com/movie.mp4?signature=REDACTED"},
    {"name":"Local","url":"http://localhost/movie.mp4"},
    {"name":"Torrent","infoHash":"not-supported"},
    {"name":"Requires-Headers","url":"https://video.example.com/locked.mp4","behaviorHints":{"proxyHeaders":{"request":{"Authorization":"private"}}}},
    {"name":"HLS","url":"https://video.example.com/movie.m3u8"},
    {"name":"MP4 via URL","url":"https://video.example.com/movie.mp4","behaviorHints":{"notWebReady":true}}
   ]}`), nil
		}
		return fakeJSON(`{}`), nil
	})}}
	manifest, err := client.StreamManifest(context.Background(), endpoint)
	if err != nil || manifest.Name != "Sample Source" {
		t.Fatalf("manifest: %+v %v", manifest, err)
	}
	got, err := client.Streams(context.Background(), endpoint, "movie", "tt123")
	if err != nil || len(got) != 1 || got[0].Name != "Clear MP4" {
		t.Fatalf("unsafe results accepted: %+v %v", got, err)
	}
}
func TestMediaURLPolicy(t *testing.T) {
	good := []string{
		"https://video.example.com/movie.mp4",
		"https://cdn.example.com/clip.m4v?sig=opaque-value",
	}
	for _, u := range good {
		if !ValidateMediaURL(u) {
			t.Errorf("valid source rejected")
		}
	}
	bad := []string{
		"http://video.example.com/movie.mp4",
		"https://127.0.0.1/clip.mp4",
		"https://video.example.com:8443/movie.mp4",
		"https://user:password@video.example.com/movie.mp4",
		"https://video.example.com/movie.m3u8",
		"https://video.example.com/video.mkv",
		"https://video.example.com/file.mp4#fragment",
		"file:///etc/passwd",
		"https://video.example.com/movie.mp4\nX: private",
	}
	for _, u := range bad {
		if ValidateMediaURL(u) {
			t.Errorf("unsafe source accepted")
		}
	}
}
func TestStreamManifestMissingStreamResource(t *testing.T) {
	client := &Client{client: &http.Client{Transport: fakeRoundTrip(func(req *http.Request) (*http.Response, error) {
		return fakeJSON(`{"id":"metadata-only","name":"Catalog","resources":["meta","catalog"]}`), nil
	})}}
	_, err := client.StreamManifest(context.Background(), "https://addon.example.com/manifest.json")
	if err == nil {
		t.Fatal("catalog-only addon accepted as stream provider")
	}
}

func TestConfiguredAddonResourcePath(t *testing.T) {
	manifestURL := "https://addon.example.com/qualities%3D1080p%2C720p%7Ca%3Ddub%2Cleg/manifest.json"
	var seen []string
	client := &Client{client: &http.Client{Transport: fakeRoundTrip(func(req *http.Request) (*http.Response, error) {
		seen = append(seen, req.URL.EscapedPath())
		if strings.HasSuffix(req.URL.Path, "/manifest.json") {
			return fakeJSON(`{"name":"Configured Source","id":"org.example.streams","resources":["stream"]}`), nil
		}
		return fakeJSON(`{"streams":[{"name":"Playable Sample","url":"https://media.example.com/original.mp4"}]}`), nil
	})}}
	source, err := client.StreamManifest(context.Background(), manifestURL)
	if err != nil || source.Name != "Configured Source" {
		t.Fatalf("manifest fetch failed: %v", err)
	}
	streams, err := client.Streams(context.Background(), manifestURL, "movie", "tt123")
	if err != nil || len(streams) != 1 {
		t.Fatalf("stream discovery failed: %v", err)
	}
	if len(seen) != 2 ||
		seen[0] != "/qualities%3D1080p%2C720p%7Ca%3Ddub%2Cleg/manifest.json" ||
		seen[1] != "/qualities%3D1080p%2C720p%7Ca%3Ddub%2Cleg/stream/movie/tt123.json" {
		t.Fatal("addon configuration was lost when forming Stremio resource path")
	}
}
