package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/isyuricunha/YuFlix/services/core/internal/catalog"
	"github.com/isyuricunha/YuFlix/services/core/internal/db"
	"github.com/jackc/pgx/v5"
)

type videoTicket struct {
	sourceID  string
	userID    string
	transport string
	url       string
	expires   time.Time
}
type videoTickets struct {
	mu    sync.Mutex
	items map[string]videoTicket
}
type displayStream struct {
	SourceID   string `json:"sourceId"`
	SourceName string `json:"sourceName"`
	Index      int    `json:"index"`
	Name       string `json:"name"`
	Title      string `json:"title,omitempty"`
	Mode       string `json:"mode"`
	Key        string `json:"key"`
}

func (a *api) listStreamAddons(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	addons, err := a.db.ListStreamAddons(ctx)
	if err != nil {
		failure(w, 503, "provedores indisponíveis")
		return
	}
	writeJSON(w, 200, map[string]any{"sources": addons})
}
func (a *api) addStreamAddon(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ManifestURL string `json:"manifestUrl"`
	}
	if decodeJSON(r, &input) != nil {
		failure(w, 400, "JSON inválido")
		return
	}
	if _, err := catalog.ValidateManifestURL(input.ManifestURL); err != nil {
		failure(w, 400, "somente manifestos Stremio HTTPS públicos, sem credenciais")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 14*time.Second)
	defer cancel()
	current, err := a.db.ListStreamAddons(ctx)
	if err != nil {
		failure(w, 503, "provedores indisponíveis")
		return
	}
	if len(current) >= 8 {
		failure(w, 409, "máximo de oito addons de reprodução")
		return
	}
	for _, item := range current {
		if item.ManifestURL == input.ManifestURL {
			failure(w, 409, "provedor já cadastrado")
			return
		}
	}
	manifest, err := a.catalogClient.StreamManifest(ctx, input.ManifestURL)
	if err != nil {
		failure(w, 422, "manifesto não contém recursos de stream compatíveis")
		return
	}
	result, err := a.db.AddStreamAddon(ctx, manifest.Name+" (Vídeo)", input.ManifestURL)
	if err != nil {
		failure(w, 409, "não foi possível cadastrar o provedor")
		return
	}
	writeJSON(w, 201, map[string]any{"source": result})
}
func (a *api) removeStreamAddon(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sourceID")
	if !sourceIDRE.MatchString(id) {
		failure(w, 400, "identificador inválido")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	deleted, err := a.db.DeleteStreamAddon(ctx, id)
	if err != nil {
		failure(w, 503, "provedores indisponíveis")
		return
	}
	if !deleted {
		failure(w, 404, "provedor não encontrado")
		return
	}
	a.video.mu.Lock()
	for key, ticket := range a.video.items {
		if ticket.sourceID == id {
			delete(a.video.items, key)
		}
	}
	a.video.mu.Unlock()
	w.WriteHeader(204)
}
func (a *api) listMediaStreams(w http.ResponseWriter, r *http.Request) {
	typ, id := r.URL.Query().Get("type"), r.URL.Query().Get("id")
	if (typ != "movie" && typ != "series") || !catalog.ValidMediaID(id) {
		failure(w, 400, "identificador de vídeo inválido")
		return
	}
	select {
	case a.mediaSlots <- struct{}{}:
		defer func() { <-a.mediaSlots }()
	default:
		failure(w, 429, "muitas buscas simultâneas")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 18*time.Second)
	defer cancel()
	addons, err := a.db.ListStreamAddons(ctx)
	if err != nil {
		failure(w, 503, "provedores indisponíveis")
		return
	}
	out := make([]displayStream, 0)
	errorsCount := 0
	for _, addon := range addons {
		if !addon.Enabled {
			continue
		}
		streams, err := a.catalogClient.Streams(ctx, addon.ManifestURL, typ, id)
		if err != nil {
			errorsCount++
			continue
		}
		for index, s := range streams {
			out = append(out, displayStream{SourceID: addon.ID, SourceName: addon.Name,
				Index: index, Name: s.Name, Title: s.Title, Mode: "direct_play_candidate", Key: a.candidateKey(addon.ID, id, s.URL)})
		}
	}
	writeJSON(w, 200, map[string]any{"streams": out, "providersChecked": len(addons), "providersFailed": errorsCount})
}
func (a *api) startMediaSession(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SourceID     string `json:"sourceId"`
		Type         string `json:"type"`
		ID           string `json:"id"`
		Index        int    `json:"index"`
		CandidateKey string `json:"candidateKey"`
	}
	if err := decodeJSON(r, &input); err != nil || !sourceIDRE.MatchString(input.SourceID) ||
		(input.Type != "movie" && input.Type != "series") || !catalog.ValidMediaID(input.ID) ||
		input.Index < 0 || input.Index >= 12 || len(input.CandidateKey) != 43 {
		failure(w, 400, "seleção de fonte inválida")
		return
	}
	select {
	case a.mediaSlots <- struct{}{}:
		defer func() { <-a.mediaSlots }()
	default:
		failure(w, 429, "muitas verificações de reprodução simultâneas")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 19*time.Second)
	defer cancel()
	addon, err := a.db.StreamAddon(ctx, input.SourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		failure(w, 404, "provedor não encontrado")
		return
	}
	if err != nil || !addon.Enabled {
		failure(w, 503, "provedor indisponível")
		return
	}
	streams, err := a.catalogClient.Streams(ctx, addon.ManifestURL, input.Type, input.ID)
	if err != nil || len(streams) <= input.Index {
		failure(w, 422, "fonte de vídeo indisponível")
		return
	}
	candidate := streams[input.Index]

	if a.candidateKey(addon.ID, input.ID, candidate.URL) != input.CandidateKey {
		failure(w, 409, "a fonte mudou; atualize as opções de reprodução")
		return
	}

	if err := a.direct.Probe(ctx, candidate.URL); err != nil {
		failure(w, 422, "a fonte não passou na verificação de MP4 com Range")
		return
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		failure(w, 503, "sessão indisponível")
		return
	}
	ticket := base64.RawURLEncoding.EncodeToString(secret)
	user := r.Context().Value(contextUserKey{}).(db.User)
	a.video.mu.Lock()
	now := time.Now()
	for key, entry := range a.video.items {
		if now.After(entry.expires) {
			delete(a.video.items, key)
		}
	}
	if len(a.video.items) >= 256 {
		a.video.mu.Unlock()
		failure(w, 429, "limite de sessões de vídeo")
		return
	}
	a.video.items[ticket] = videoTicket{sourceID: input.SourceID, userID: user.ID, transport: transportFromContext(r),
		url: candidate.URL, expires: now.Add(6 * time.Hour)}
	a.video.mu.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 201, map[string]any{
		"path": "/api/v1/media/play/" + ticket,
		"mode": "direct_play", "status": "probe_passed_not_browser_verified",
		"sourceName": addon.Name, "name": candidate.Name,
	})
}
func (a *api) relayMedia(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if len(token) != 43 {
		failure(w, 404, "sessão de reprodução não encontrada")
		return
	}
	user := r.Context().Value(contextUserKey{}).(db.User)
	a.video.mu.Lock()
	ticket, exists := a.video.items[token]
	if exists && time.Now().After(ticket.expires) {
		delete(a.video.items, token)
		exists = false
	}
	a.video.mu.Unlock()
	if !exists || ticket.userID != user.ID || ticket.transport != transportFromContext(r) {
		failure(w, 404, "sessão de reprodução não encontrada")
		return
	}
	select {
	case a.relaySlots <- struct{}{}:
		defer func() { <-a.relaySlots }()
	default:
		failure(w, 429, "limite de transmissões simultâneas")
		return
	}
	// Never return or log upstream source URLs or request headers.
	a.direct.Relay(w, r, ticket.url)
}

// Provider ordering can change between search and selection. Bind each candidate
// to its exact upstream URL without exposing the address or query signature.
func (a *api) candidateKey(sourceID, videoID, url string) string {
	h := hmac.New(sha256.New, a.streamKey[:])
	h.Write([]byte(sourceID))
	h.Write([]byte{0})
	h.Write([]byte(videoID))
	h.Write([]byte{0})
	h.Write([]byte(url))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
