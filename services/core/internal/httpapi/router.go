package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type systemInfo struct {
	Name                string `json:"name"`
	Slogan              string `json:"slogan"`
	APIVersion          string `json:"apiVersion"`
	Phase               string `json:"phase"`
	PlaybackReady       bool   `json:"playbackReady"`
	DatabaseReady       bool   `json:"databaseReady"`
	AuthenticationReady bool   `json:"authenticationReady"`
}

func NewRouter(options ...Options) http.Handler {
	o := Options{}
	if len(options) > 0 {
		o = options[0]
	}
	a := newAPI(o)
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
	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		if a.db != nil {
			ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
			defer cancel()
			if err := a.db.Ping(ctx); err != nil {
				failure(w, 503, "banco indisponível")
				return
			}
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ready\n"))
	})
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/system/info", func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(systemInfo{
				Name: "YuFlix", Slogan: "I Watch What I Want.",
				APIVersion: "v1", Phase: "foundation", PlaybackReady: false,
				DatabaseReady: a.db != nil, AuthenticationReady: a.authenticationReady(req),
			})
		})
		r.Group(func(r chi.Router) {
			r.Use(a.requireTransport)
			r.Post("/auth/login", a.login)
			r.Group(func(r chi.Router) {
				r.Use(a.withUser)
				r.Get("/auth/me", a.me)
				r.Post("/auth/logout", a.logout)
				r.Get("/catalog/sources", a.listCatalogSources)
				r.Get("/catalog/sources/{sourceID}/items", a.catalogItems)
				r.Get("/catalog/sources/{sourceID}/meta", a.catalogMeta)
				r.Post("/media/auto", a.autoMediaSession)

				r.Get("/media/play/{token}", a.relayMedia)
				r.Route("/admin", func(r chi.Router) {
					r.Use(a.adminOnly)
					r.Get("/users", a.listUsers)
					r.Post("/users", a.createUser)
					r.Get("/settings", a.getSettings)
					r.Put("/settings/{key}", a.putSetting)
					r.Post("/catalog/sources", a.addCatalogSource)
					r.Delete("/catalog/sources/{sourceID}", a.deleteCatalogSource)
					r.Put("/catalog/sources/{sourceID}/cache", a.updateCatalogCacheMode)
					r.Put("/catalog/cache", a.updateAllCatalogCacheModes)
					r.Get("/media/cache", a.getMediaCachePolicy)
					r.Put("/media/cache", a.putMediaCachePolicy)
					r.Get("/media/addons", a.listStreamAddons)
					r.Get("/media/streams", a.listMediaStreams)
					r.Post("/media/sessions", a.startMediaSession)
					r.Post("/media/addons", a.addStreamAddon)
					r.Delete("/media/addons/{sourceID}", a.removeStreamAddon)
					r.Put("/media/addons/{sourceID}/route", a.updateStreamAddonRoute)
					r.Put("/media/addons/{sourceID}/policy", a.updateStreamAddonPolicy)
					r.Get("/network/egress", a.streamEgressOptions)
				})
			})
		})
	})
	return r
}
