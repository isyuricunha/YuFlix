package main

import (
	"context"
	"encoding/json"
	"github.com/isyuricunha/YuFlix/services/core/internal/catalog"
	"net/http"
	"net/url"
	"time"
)

// Probe permits only one of the MP4 URLs returned by an allowlisted addon,
// and only while the corresponding discovery result is still fresh.
func (h *relay) probeMedia(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 6500)
	var data map[string]string
	if json.NewDecoder(r.Body).Decode(&data) != nil || len(data) != 4 {
		http.Error(w, "invalid request", 400)
		return
	}
	manifest, err := catalog.ValidateManifestURL(data["manifestUrl"])
	typ, id, urlString := data["type"], data["id"], data["url"]
	if err != nil || (typ != "movie" && typ != "series") || !catalog.ValidMediaID(id) || !catalog.ValidateMediaURL(urlString) {
		http.Error(w, "invalid input", 400)
		return
	}
	u, _ := url.Parse(manifest)
	if !h.hosts[u.Hostname()] {
		http.Error(w, "not allowed", 403)
		return
	}
	key := manifest + "|" + typ + "|" + id
	h.mu.Lock()
	existing, ok := h.cache[key]
	h.mu.Unlock()
	if !ok || time.Now().After(existing.expires) {
		http.Error(w, "discovery expired", 409)
		return
	}
	match := false
	for _, candidate := range existing.report.Streams {
		if candidate.URL == urlString {
			match = true
			break
		}
	}
	if !match {
		http.Error(w, "source not discovered", 403)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 11*time.Second)
	defer cancel()
	if err := h.direct.Probe(ctx, urlString); err != nil {
		http.Error(w, "MP4 Range probe failed", 422)
		return
	}
	h.mu.Lock()
	if len(h.allowed) > 128 {
		next := map[string]time.Time{}
		for k, expiry := range h.allowed {
			if time.Now().Before(expiry) {
				next[k] = expiry
			}
		}
		if len(next) > 128 {
			next = map[string]time.Time{}
		}
		h.allowed = next
	}
	h.allowed[urlString] = time.Now().Add(6 * time.Hour)
	h.mu.Unlock()
	send(w, 200, map[string]any{"valid": true})
}
func (h *relay) streamMedia(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2200)
	var data map[string]string
	if json.NewDecoder(r.Body).Decode(&data) != nil || len(data) != 1 ||
		!catalog.ValidateMediaURL(data["url"]) {
		http.Error(w, "invalid request", 400)
		return
	}
	u := data["url"]
	h.mu.Lock()
	expiry, ok := h.allowed[u]
	h.mu.Unlock()
	if !ok || time.Now().After(expiry) {
		http.Error(w, "media session unavailable", 404)
		return
	}
	select {
	case h.videoSlots <- struct{}{}:
		defer func() { <-h.videoSlots }()
	default:
		http.Error(w, "relay busy", 429)
		return
	}
	h.direct.Relay(w, r, u)
}
