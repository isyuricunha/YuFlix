package media

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func mediaResp(code int, ct string, body string) *http.Response {
	h := http.Header{}
	h.Set("Content-Type", ct)
	h.Set("Set-Cookie", "upstream-secret=private")
	h.Set("Location", "https://private.example.com/private")
	h.Set("Content-Range", "bytes 0-1023/9999")
	return &http.Response{StatusCode: code, Header: h, Body: io.NopCloser(strings.NewReader(body))}
}
func TestProbeRangeAndMP4Signature(t *testing.T) {
	d := &Direct{HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Range"); got != "bytes=0-1023" {
			t.Errorf("missing bounded Range: %q", got)
		}
		if req.Header.Get("Cookie") != "" {
			t.Error("client cookie forwarded")
		}
		return mediaResp(206, "video/mp4", "\x00\x00\x00\x18ftypisom"+strings.Repeat("a", 1012)), nil
	})}}
	if err := d.Probe(context.Background(), "https://video.example.com/movie.mp4?signature=secret"); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		code     int
		ct, body string
	}{
		{200, "video/mp4", "\x00\x00\x00\x18ftypisom"},
		{206, "text/html", "<html>not a video</html>"},
		{206, "video/mp4", "not-an-mp4-file"},
	} {
		d.HTTP = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return mediaResp(scenario.code, scenario.ct, scenario.body), nil
		})}
		if err := d.Probe(context.Background(), "https://video.example.com/movie.mp4"); err == nil {
			t.Errorf("bad probe passed: %+v", scenario)
		}
	}
}
func TestRelayRangeAndPrivacy(t *testing.T) {
	d := &Direct{HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Range") != "bytes=100-199" {
			t.Errorf("Range not forwarded: %q", req.Header.Get("Range"))
		}
		if req.Header.Get("Cookie") != "" {
			t.Error("session cookie forwarded")
		}
		return mediaResp(206, "video/mp4", "synthetic media segment"), nil
	})}}
	req := httptest.NewRequest("GET", "/stream", nil)
	req.Header.Set("Range", "bytes=100-199")
	req.Header.Set("Cookie", "yuflix_http_session=private")
	w := httptest.NewRecorder()
	d.Relay(w, req, "https://video.example.com/movie.mp4?signature=must-not-leak")
	if w.Code != 206 {
		t.Fatalf("expected 206 got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Set-Cookie") != "" || w.Header().Get("Location") != "" {
		t.Fatal("private upstream headers leaked")
	}
	if w.Header().Get("Content-Type") != "video/mp4" {
		t.Fatal("incorrect video content type")
	}
	if strings.Contains(w.Body.String(), "must-not-leak") {
		t.Fatal("signed source URL leaked")
	}
	bad := httptest.NewRequest("GET", "/stream", nil)
	bad.Header.Set("Range", "bytes=0-50,100-200")
	denied := httptest.NewRecorder()
	d.Relay(denied, bad, "https://video.example.com/movie.mp4")
	if denied.Code != 416 {
		t.Fatalf("multi-range request accepted: %d", denied.Code)
	}
}
