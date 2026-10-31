package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"regexp"
)

func (c *Client) UsesEgress(manifestURL string) bool {
	return c != nil && c.egress != nil && c.egress.handles(manifestURL)
}
func (c *Client) ProbeViaEgress(ctx context.Context, manifestURL, typ, id, sourceURL string) error {
	if !c.UsesEgress(manifestURL) || !validateMediaURL(sourceURL) {
		return ErrUnsafeURL
	}
	body, err := json.Marshal(map[string]string{
		"manifestUrl": manifestURL, "type": typ, "id": id, "url": sourceURL})
	if err != nil {
		return ErrInvalid
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.egress.endpoint+"/v1/probe", bytes.NewReader(body))
	if err != nil {
		return ErrUpstream
	}
	req.Header.Set("Authorization", "Bearer "+c.egress.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.egress.client.Do(req)
	if err != nil {
		return ErrUpstream
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return &UpstreamHTTPError{StatusCode: res.StatusCode}
	}
	return nil
}

var egressRangePattern = regexp.MustCompile("^[b]ytes=([0-9]{1,18}-[0-9]{0,18}|-[0-9]{1,18})$")

func (c *Client) RelayViaEgress(w http.ResponseWriter, r *http.Request, sourceURL string) {
	if c == nil || c.egress == nil || !validateMediaURL(sourceURL) {
		http.Error(w, "reprodução indisponível", 502)
		return
	}
	rng := r.Header.Get("Range")
	if rng != "" && !egressRangePattern.MatchString(rng) {
		http.Error(w, "intervalo inválido", 416)
		return
	}
	data, err := json.Marshal(map[string]string{"url": sourceURL})
	if err != nil {
		http.Error(w, "reprodução indisponível", 502)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, c.egress.endpoint+"/v1/media", bytes.NewReader(data))
	if err != nil {
		http.Error(w, "reprodução indisponível", 502)
		return
	}
	req.Header.Set("Authorization", "Bearer "+c.egress.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "identity")
	if rng != "" {
		req.Header.Set("Range", rng)
	}
	// Streaming requests cannot have a whole-response deadline.
	client := &http.Client{Transport: c.egress.client.Transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		http.Error(w, "fonte indisponível", 502)
		return
	}
	defer res.Body.Close()
	if (rng != "" && res.StatusCode != 206) || (rng == "" && res.StatusCode != 200 && res.StatusCode != 206) {
		http.Error(w, "reprodução indisponível", 502)
		return
	}
	kind, _, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if err != nil || kind != "video/mp4" {
		http.Error(w, "formato incompatível", 415)
		return
	}
	if rng != "" && res.Header.Get("Content-Range") == "" {
		http.Error(w, "intervalo indisponível", 502)
		return
	}
	for _, key := range []string{"Content-Length", "Content-Range", "ETag", "Last-Modified"} {
		if value := res.Header.Get(key); value != "" {
			w.Header().Set(key, value)
		}
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, res.Body)
}
