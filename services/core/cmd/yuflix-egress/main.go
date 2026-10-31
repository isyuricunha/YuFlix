package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"github.com/isyuricunha/YuFlix/services/core/internal/catalog"
	"github.com/isyuricunha/YuFlix/services/core/internal/media"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type cacheEntry struct {
	report  catalog.StreamReport
	expires time.Time
}
type relay struct {
	token      string
	hosts      map[string]bool
	client     *catalog.Client
	slots      chan struct{}
	mu         sync.Mutex
	cache      map[string]cacheEntry
	allowed    map[string]time.Time
	videoSlots chan struct{}
	direct     *media.Direct
}

func (h *relay) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Path == "/healthz" && r.Method == "GET" {
		w.WriteHeader(200)
		return
	}
	if r.URL.Path != "/v1/streams" && r.URL.Path != "/v1/probe" && r.URL.Path != "/v1/media" {
		http.Error(w, "not found", 404)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	token := "Bearer " + h.token
	value := r.Header.Get("Authorization")
	if len(value) != len(token) || subtle.ConstantTimeCompare([]byte(value), []byte(token)) != 1 {
		http.Error(w, "unauthorized", 401)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "wrong content type", 415)
		return
	}
	if r.URL.Path == "/v1/probe" {
		h.probeMedia(w, r)
		return
	}
	if r.URL.Path == "/v1/media" {
		h.streamMedia(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 6500)
	var v map[string]string
	d := json.NewDecoder(r.Body)
	if d.Decode(&v) != nil || len(v) != 3 {
		http.Error(w, "invalid request", 400)
		return
	}
	var extra any
	if d.Decode(&extra) == nil {
		http.Error(w, "invalid request", 400)
		return
	}
	manifest, err := catalog.ValidateManifestURL(v["manifestUrl"])
	typ, id := v["type"], v["id"]
	if err != nil || (typ != "movie" && typ != "series") || !catalog.ValidMediaID(id) {
		http.Error(w, "invalid input", 400)
		return
	}
	u, _ := url.Parse(manifest)
	if !h.hosts[u.Hostname()] {
		http.Error(w, "not allowed", 403)
		return
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		http.Error(w, "busy", 429)
		return
	}
	key := manifest + "|" + typ + "|" + id
	h.mu.Lock()
	previous, hit := h.cache[key]
	h.mu.Unlock()
	if hit && time.Now().Before(previous.expires) {
		send(w, 200, map[string]any{"report": previous.report})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	report, err := h.client.StreamsDetailed(ctx, manifest, typ, id)
	if err != nil {
		var upstream *catalog.UpstreamHTTPError
		if errors.As(err, &upstream) {
			send(w, 502, map[string]any{"status": upstream.StatusCode})
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			send(w, 504, map[string]any{"reason": "timeout"})
			return
		}
		send(w, 502, map[string]any{"reason": "upstream_unavailable"})
		return
	}
	h.mu.Lock()
	if len(h.cache) > 48 {
		h.cache = map[string]cacheEntry{}
	}
	h.cache[key] = cacheEntry{report, time.Now().Add(45 * time.Second)}
	h.mu.Unlock()
	send(w, 200, map[string]any{"report": report})
}
func send(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func main() {
	token := os.Getenv("YUFLIX_EGRESS_TOKEN")
	addr := os.Getenv("YUFLIX_EGRESS_ADDR")
	if len(token) < 32 || len(token) > 128 || addr == "" {
		panic("invalid egress configuration")
	}
	hosts := map[string]bool{}
	for _, host := range strings.Split(os.Getenv("YUFLIX_EGRESS_ALLOWED_HOSTS"), ",") {
		host = strings.TrimSpace(host)
		if host == "" || host != strings.ToLower(host) || strings.ContainsAny(host, ":/?@") {
			panic("invalid egress host")
		}
		hosts[host] = true
	}
	if len(hosts) == 0 || len(hosts) > 12 {
		panic("invalid egress hosts")
	}
	h := &relay{token: token, hosts: hosts, client: catalog.NewClient(), slots: make(chan struct{}, 3), cache: map[string]cacheEntry{}, allowed: map[string]time.Time{},
		videoSlots: make(chan struct{}, 6), direct: media.NewDirect()}
	server := &http.Server{Addr: addr, Handler: http.HandlerFunc(h.serve), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 18 * time.Second, WriteTimeout: 0, IdleTimeout: 20 * time.Second, MaxHeaderBytes: 8192}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	slog.Info("YuFlix egress starting")
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			os.Exit(1)
		}
	case <-ctx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}
}
