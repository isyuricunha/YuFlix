package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/isyuricunha/YuFlix/services/core/internal/auth"
	"github.com/isyuricunha/YuFlix/services/core/internal/db"
)

func TestIntegrationPostgres17(t *testing.T) {
	dsn := os.Getenv("YUFLIX_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated PostgreSQL 17 test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	store, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatal("non-idempotent migration", err)
	}
	if !store.SchemaReady(ctx) {
		t.Fatal("schema not ready")
	}
	hash, err := auth.HashPassword("A long test admin password")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := store.CreateFirstAdmin(ctx, "admin", hash)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateFirstAdmin(ctx, "another", hash); err == nil {
		t.Fatal("bootstrap allowed twice")
	}
	a := NewRouter(Options{Database: store, PublicOrigin: "https://stream.example.com", TrustProxy: true})
	call := func(method, path, body, cookie, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Host = "stream.example.com"
		req.Header.Set("X-Forwarded-Proto", "https")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		rec := httptest.NewRecorder()
		a.ServeHTTP(rec, req)
		return rec
	}
	loginJSON := `{"username":"admin","password":"A long test admin password"}`
	if resp := call("POST", "/api/v1/auth/login", loginJSON, "", "https://evil.example.com"); resp.Code != 403 {
		t.Fatalf("cross origin status %d", resp.Code)
	}
	if resp := call("POST", "/api/v1/auth/login", loginJSON, "", ""); resp.Code != 403 {
		t.Fatalf("missing origin status %d", resp.Code)
	}
	resp := call("POST", "/api/v1/auth/login", loginJSON, "", "https://stream.example.com")
	if resp.Code != 200 {
		t.Fatalf("admin login status %d: %s", resp.Code, resp.Body.String())
	}
	cookies := resp.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != cookieName || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie flags invalid: %+v", cookies)
	}
	adminCookie := cookies[0].Name + "=" + cookies[0].Value
	if resp = call("GET", "/api/v1/auth/me", "", adminCookie, ""); resp.Code != 200 {
		t.Fatalf("me %d", resp.Code)
	}
	if response := call("GET", "/api/v1/admin/catalog/providers", "", adminCookie, ""); response.Code != 200 ||
		!strings.Contains(response.Body.String(), `"id":"tmdb"`) ||
		!strings.Contains(response.Body.String(), `"id":"tmdb-collections"`) ||
		strings.Contains(response.Body.String(), "manifest.json") {
		t.Fatal("unsafe or missing curated catalog provider listing")
	}
	if response := call("POST", "/api/v1/admin/catalog/providers/unknown", "", adminCookie,
		"https://stream.example.com"); response.Code != 404 {
		t.Fatal("unknown provider was accepted")
	}
	if resp = call("GET", "/api/v1/admin/users", "", adminCookie, ""); resp.Code != 200 {
		t.Fatalf("list users %d", resp.Code)
	}
	if resp = call("POST", "/api/v1/admin/users", `{"username":"gustavo","password":"A long new user password","role":"member"}`, adminCookie, "https://stream.example.com"); resp.Code != 201 {
		t.Fatalf("create user %d: %s", resp.Code, resp.Body.String())
	}
	if resp = call("PUT", "/api/v1/admin/settings/catalog.default_language", `{"value":"pt-BR"}`, adminCookie, "https://stream.example.com"); resp.Code != 200 {
		t.Fatalf("settings write %d: %s", resp.Code, resp.Body.String())
	}
	if resp = call("GET", "/api/v1/admin/settings", "", adminCookie, ""); resp.Code != 200 {
		t.Fatalf("settings read %d: %s", resp.Code, resp.Body.String())
	}
	var settings struct {
		Settings map[string]string `json:"settings"`
	}
	if err = json.Unmarshal(resp.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.Settings["catalog.default_language"] != "pt-BR" {
		t.Fatal("settings not persisted")
	}
	memberLogin := call("POST", "/api/v1/auth/login", `{"username":"gustavo","password":"A long new user password"}`, "", "https://stream.example.com")
	if memberLogin.Code != 200 {
		t.Fatalf("member login %d", memberLogin.Code)
	}
	if response := call("GET", "/api/v1/admin/media/cache", "", adminCookie, ""); response.Code != 200 ||
		!strings.Contains(response.Body.String(), `"mode":"automatic"`) ||
		!strings.Contains(response.Body.String(), `"active":false`) {
		t.Fatalf("invalid media cache defaults: %d: %s", response.Code, response.Body.String())
	}
	if response := call("PUT", "/api/v1/admin/media/cache",
		`{"policy":{"mode":"manual","storage":"disk","maxMiB":50000,"maxConcurrent":2,"segmentSeconds":3,"idleSeconds":90,"cleanupGraceSeconds":15}}`,
		adminCookie, "https://stream.example.com"); response.Code != 400 {
		t.Fatalf("unsafe disk size accepted: %d", response.Code)
	}
	if response := call("PUT", "/api/v1/admin/media/cache",
		`{"policy":{"mode":"manual","storage":"disk","maxMiB":512,"maxConcurrent":2,"segmentSeconds":4,"idleSeconds":120,"cleanupGraceSeconds":0}}`,
		adminCookie, "https://stream.example.com"); response.Code != 200 ||
		!strings.Contains(response.Body.String(), `"maxMiB":512`) {
		t.Fatalf("media policy write failed: %d %s", response.Code, response.Body.String())
	}
	if response := call("GET", "/api/v1/admin/media/cache", "", adminCookie, ""); response.Code != 200 ||
		!strings.Contains(response.Body.String(), `"maxMiB":512`) {
		t.Fatal("media settings not persisted")
	}

	memberCookie := memberLogin.Result().Cookies()[0]
	if response := call("GET", "/api/v1/admin/media/cache", "", memberCookie.Name+"="+memberCookie.Value, ""); response.Code != 403 {
		t.Fatal("member accessed media cache settings")
	}
	if response := call("GET", "/api/v1/admin/catalog/providers", "", memberCookie.Name+"="+memberCookie.Value, ""); response.Code != 403 {
		t.Fatal("member accessed provider integration registry")
	}
	if resp = call("GET", "/api/v1/admin/users", "", memberCookie.Name+"="+memberCookie.Value, ""); resp.Code != 403 {
		t.Fatalf("member accessed admin %d", resp.Code)
	}
	// Sources are shared while permissions stay scoped to the current session.
	source, err := store.AddCatalogSource(ctx, "Public Catalog", "https://v3-cinemeta.strem.io/manifest.json",
		[]db.StoredCatalog{{Type: "movie", ID: "top", Name: "Popular"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"anime", "collections"} {
		if err := store.SaveCatalogSnapshot(ctx, source.ID, kind, "top", []byte(`[]`)); err != nil {
			t.Fatalf("new catalog kind cache unavailable: %s (%v)", kind, err)
		}
	}
	if err := store.SaveCatalogSnapshot(ctx, source.ID, "movie", "top",
		[]byte(`[{"id":"tt123","type":"movie","name":"Cached"}]`)); err != nil {
		t.Fatal("cannot persist catalog snapshot", err)
	}
	matches, searchErr := store.SearchCachedTitles(ctx, "Cached")
	if searchErr != nil || len(matches) != 1 || matches[0].SourceID != source.ID {
		t.Fatalf("cached title search failed: %+v (%v)", matches, searchErr)
	}
	if response := call("GET", "/api/v1/catalog/search?q=Cached", "",
		memberCookie.Name+"="+memberCookie.Value, ""); response.Code != 200 ||
		!strings.Contains(response.Body.String(), `"scope":"cached_catalogs"`) {
		t.Fatalf("member cached catalog search failed: %d", response.Code)
	}
	if response := call("GET", "/api/v1/catalog/search?q=x", "",
		memberCookie.Name+"="+memberCookie.Value, ""); response.Code != 400 {
		t.Fatal("invalid short search accepted")
	}
	if cached, _, err := store.CatalogSnapshot(ctx, source.ID, "movie", "top"); err != nil || !strings.Contains(string(cached), "Cached") {
		t.Fatal("catalog snapshot not readable")
	}
	if changed, err := store.SetCatalogCacheMode(ctx, source.ID, "off"); err != nil || changed.CacheMode != "off" {
		t.Fatal("could not disable source cache", err)
	}
	if _, _, err := store.CatalogSnapshot(ctx, source.ID, "movie", "top"); err == nil {
		t.Fatal("disabled cache retained persisted catalog")
	}
	response := call("GET", "/api/v1/catalog/sources", "", memberCookie.Name+"="+memberCookie.Value, "")
	if response.Code != 200 || strings.Contains(response.Body.String(), "v3-cinemeta.strem.io") {
		t.Fatalf("catalog sources inaccessible or manifest URL exposed: %d", response.Code)
	}
	if response = call("DELETE", "/api/v1/admin/catalog/sources/"+source.ID, "",
		memberCookie.Name+"="+memberCookie.Value, "https://stream.example.com"); response.Code != 403 {
		t.Fatalf("member deleted shared catalog: %d", response.Code)
	}
	if response = call("DELETE", "/api/v1/admin/catalog/sources/"+source.ID, "",
		adminCookie, "https://stream.example.com"); response.Code != 204 {
		t.Fatalf("admin could not remove shared catalog: %d", response.Code)
	}
	// Stream providers are shared, but only administrators may configure them.
	streamAddon, err := store.AddStreamAddon(ctx, "Sample Streams", "https://provider.example.com/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if resp = call("GET", "/api/v1/media/addons", "",
		memberCookie.Name+"="+memberCookie.Value, ""); resp.Code != 404 {
		t.Fatalf("member accessed provider configuration: %d", resp.Code)
	}
	if resp = call("DELETE", "/api/v1/admin/media/addons/"+streamAddon.ID, "",
		memberCookie.Name+"="+memberCookie.Value, "https://stream.example.com"); resp.Code != 403 {
		t.Fatalf("member removed stream source: %d", resp.Code)
	}
	if streamAddon.NetworkRoute != "direct" {
		t.Fatal("new addons must use direct transport")
	}
	if streamAddon.Priority != 500 || streamAddon.TimeoutMs != 5000 {
		t.Fatal("unsafe source default policy")
	}
	policyPath := "/api/v1/admin/media/addons/" + streamAddon.ID + "/policy"
	if resp = call("PUT", policyPath, `{"priority":0,"timeoutMs":5000}`,
		adminCookie, "https://stream.example.com"); resp.Code != 400 {
		t.Fatal("accepted invalid priority")
	}
	if resp = call("PUT", policyPath, `{"priority":12,"timeoutMs":7000}`,
		adminCookie, "https://stream.example.com"); resp.Code != 200 ||
		!strings.Contains(resp.Body.String(), `"priority":12`) {
		t.Fatal("failed to save stream policy")
	}
	memberCookieString := memberCookie.Name + "=" + memberCookie.Value
	routePath := "/api/v1/admin/media/addons/" + streamAddon.ID + "/route"
	if resp = call("PUT", routePath, `{"route":"auto"}`,
		memberCookieString, "https://stream.example.com"); resp.Code != 403 {
		t.Fatalf("member changed source network mode: %d", resp.Code)
	}
	if resp = call("PUT", routePath, `{"route":"invalid"}`,
		adminCookie, "https://stream.example.com"); resp.Code != 400 {
		t.Fatalf("invalid routing mode accepted: %d", resp.Code)
	}
	if resp = call("PUT", routePath, `{"route":"egress"}`,
		adminCookie, "https://stream.example.com"); resp.Code != 409 {
		t.Fatalf("unconfigured egress accepted: %d", resp.Code)
	}
	if resp = call("PUT", routePath, `{"route":"direct"}`,
		adminCookie, "https://stream.example.com"); resp.Code != 200 ||
		!strings.Contains(resp.Body.String(), `"networkRoute":"direct"`) {
		t.Fatalf("direct routing could not be saved: %d", resp.Code)
	}
	if resp = call("GET", "/api/v1/admin/network/egress", "", adminCookie, ""); resp.Code != 200 ||
		strings.Contains(resp.Body.String(), "manifestUrl") {
		t.Fatalf("egress API exposed source settings: %d", resp.Code)
	}
	if resp = call("POST", "/api/v1/media/sessions",
		`{"sourceId":"invalid","type":"movie","id":"tt123","index":0}`,
		memberCookie.Name+"="+memberCookie.Value, "https://stream.example.com"); resp.Code != 404 {
		t.Fatalf("member access to manual sessions was accepted: %d", resp.Code)
	}
	if resp = call("DELETE", "/api/v1/admin/media/addons/"+streamAddon.ID, "",
		adminCookie, "https://stream.example.com"); resp.Code != 204 {
		t.Fatalf("admin removal failed: %d", resp.Code)
	}
	if resp = call("POST", "/api/v1/auth/logout", "", adminCookie, "https://stream.example.com"); resp.Code != 204 {
		t.Fatalf("logout %d", resp.Code)
	}
	if resp = call("GET", "/api/v1/auth/me", "", adminCookie, ""); resp.Code != 401 {
		t.Fatalf("revocation failed %d", resp.Code)
	}
	// Run HTTP and HTTPS against the same database to prove transport separation.
	dual := NewRouter(Options{
		Database: store, PublicOrigin: "https://stream.example.com",
		AllowedOrigins:    []string{"http://stream.example.com:62471"},
		AllowInsecureHTTP: true, TrustProxy: true,
	})
	callHTTP := func(method, path, body, cookie, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://stream.example.com:62471"+path, strings.NewReader(body))
		req.Host = "stream.example.com:62471"
		req.Header.Set("X-Forwarded-Proto", "http")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		rec := httptest.NewRecorder()
		dual.ServeHTTP(rec, req)
		return rec
	}
	httpOrigin := "http://stream.example.com:62471"
	httpLogin := callHTTP("POST", "/api/v1/auth/login", loginJSON, "", httpOrigin)
	if httpLogin.Code != 200 {
		t.Fatalf("HTTP login status %d: %s", httpLogin.Code, httpLogin.Body.String())
	}
	httpCookies := httpLogin.Result().Cookies()
	if len(httpCookies) != 1 || httpCookies[0].Name != httpCookieName || httpCookies[0].Secure || !httpCookies[0].HttpOnly {
		t.Fatalf("HTTP cookie attributes invalid %+v", httpCookies)
	}
	httpCookie := httpCookies[0].Name + "=" + httpCookies[0].Value
	if v := callHTTP("GET", "/api/v1/auth/me", "", httpCookie, ""); v.Code != 200 {
		t.Fatalf("HTTP session failed %d", v.Code)
	}
	if v := callHTTP("GET", "/api/v1/auth/me", "", memberCookie.Name+"="+memberCookie.Value, ""); v.Code != 401 {
		t.Fatalf("HTTPS cookie accepted over HTTP %d", v.Code)
	}
	if v := callHTTP("POST", "/api/v1/auth/logout", "", httpCookie, "https://stream.example.com"); v.Code != 403 {
		t.Fatalf("cross-protocol Origin accepted %d", v.Code)
	}
	if v := callHTTP("POST", "/api/v1/auth/logout", "", httpCookie, httpOrigin); v.Code != 204 {
		t.Fatalf("HTTP logout %d", v.Code)
	}
	if v := callHTTP("GET", "/api/v1/auth/me", "", httpCookie, ""); v.Code != 401 {
		t.Fatalf("HTTP session not revoked %d", v.Code)
	}
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.Host = "stream.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("Cookie", httpCookie)
	rec := httptest.NewRecorder()
	dual.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("HTTP cookie accepted under HTTPS: %d", rec.Code)
	}

	deniedHTTP := NewRouter(Options{Database: store, AllowedOrigins: []string{httpOrigin}, TrustProxy: true})
	request := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(loginJSON))
	request.Host = "stream.example.com:62471"
	request.Header.Set("X-Forwarded-Proto", "http")
	request.Header.Set("Origin", httpOrigin)
	request.Header.Set("Content-Type", "application/json")
	denied := httptest.NewRecorder()
	deniedHTTP.ServeHTTP(denied, request)
	if denied.Code != 403 {
		t.Fatalf("HTTP accidentally enabled by default: %d", denied.Code)
	}
	sha := sha256.Sum256([]byte(admin.ID))
	if len(base64.RawURLEncoding.EncodeToString(sha[:])) == 0 {
		t.Fatal("invalid token helper")
	}
}
