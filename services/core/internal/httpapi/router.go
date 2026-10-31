package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type systemInfo struct {
	Name          string `json:"name"`
	Slogan        string `json:"slogan"`
	APIVersion    string `json:"apiVersion"`
	Phase         string `json:"phase"`
	PlaybackReady bool   `json:"playbackReady"`
}

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("Cache-Control", "no-store")
			next.ServeHTTP(w, req)
		})
	})
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ready\n"))
	})
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/system/info", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(systemInfo{
				Name: "YuFlix", Slogan: "I Watch What I Want.",
				APIVersion: "v1", Phase: "foundation", PlaybackReady: false,
			})
		})
	})
	return r
}
