package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sort"
	"time"

	"github.com/isyuricunha/YuFlix/services/core/internal/catalog"
	"github.com/isyuricunha/YuFlix/services/core/internal/db"
)

func (a *api) autoMediaSession(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Type    string `json:"type"`
		ID      string `json:"id"`
		Attempt int    `json:"attempt"`
	}
	if decodeJSON(r, &input) != nil || (input.Type != "movie" && input.Type != "series") || !catalog.ValidMediaID(input.ID) || input.Attempt < 0 || input.Attempt > 7 {
		failure(w, 400, "reprodução inválida")
		return
	}
	select {
	case a.mediaSlots <- struct{}{}:
		defer func() { <-a.mediaSlots }()
	default:
		failure(w, 429, "muitas buscas simultâneas")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 29*time.Second)
	defer cancel()
	addons, err := a.db.ListStreamAddons(ctx)
	if err != nil {
		failure(w, 503, "fontes indisponíveis")
		return
	}
	active := make([]db.StreamAddon, 0, len(addons))
	for _, s := range addons {
		if s.Enabled {
			active = append(active, s)
		}
	}
	if len(active) == 0 {
		failure(w, 404, "nenhuma fonte cadastrada")
		return
	}
	sort.SliceStable(active, func(i, j int) bool { return active[i].Priority < active[j].Priority })
	results := a.discoverAll(ctx, input.Type, input.ID, active)
	count := 0
	for i, addon := range active {
		var found autoResult
		select {
		case found = <-results[i]:
		case <-ctx.Done():
			failure(w, 504, "busca excedeu o tempo limite")
			return
		}
		if found.err != nil {
			continue
		}
		for _, candidate := range found.report.Streams {
			remote := found.remote
			if count < input.Attempt {
				count++
				continue
			}
			count++
			probeCtx, done := context.WithTimeout(ctx, 8*time.Second)
			var probeErr error
			if remote {
				probeErr = a.catalogClient.ProbeViaEgress(probeCtx, addon.ManifestURL, input.Type, input.ID, candidate.URL)
			} else {
				probeErr = a.direct.Probe(probeCtx, candidate.URL)
			}
			done()
			if probeErr != nil {
				continue
			}
			token, err := a.issueVideoTicket(r, addon.ID, candidate.URL, remote)
			if err != nil {
				failure(w, 503, "sessão indisponível")
				return
			}
			writeJSON(w, 201, map[string]any{"path": "/api/v1/media/play/" + token, "mode": "direct_play",
				"status": "probe_passed_not_browser_verified", "name": "Reprodução YuFlix", "sourceName": "", "nextAttempt": count})
			return
		}
	}
	failure(w, 422, "nenhuma fonte compatível foi encontrada")
}

func (a *api) issueVideoTicket(r *http.Request, sourceID, url string, remote bool) (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	user := r.Context().Value(contextUserKey{}).(db.User)
	a.video.mu.Lock()
	defer a.video.mu.Unlock()
	now := time.Now()
	for key, item := range a.video.items {
		if now.After(item.expires) {
			delete(a.video.items, key)
		}
	}
	if len(a.video.items) >= 256 {
		return "", context.DeadlineExceeded
	}
	a.video.items[token] = videoTicket{sourceID: sourceID, userID: user.ID, transport: transportFromContext(r),
		url: url, remote: remote, expires: now.Add(6 * time.Hour)}
	return token, nil
}
