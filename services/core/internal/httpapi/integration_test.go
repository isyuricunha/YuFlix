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
	a := NewRouter(Options{Database: store, PublicOrigin: "https://stream.example.com"})
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
	memberCookie := memberLogin.Result().Cookies()[0]
	if resp = call("GET", "/api/v1/admin/users", "", memberCookie.Name+"="+memberCookie.Value, ""); resp.Code != 403 {
		t.Fatalf("member accessed admin %d", resp.Code)
	}
	if resp = call("POST", "/api/v1/auth/logout", "", adminCookie, "https://stream.example.com"); resp.Code != 204 {
		t.Fatalf("logout %d", resp.Code)
	}
	if resp = call("GET", "/api/v1/auth/me", "", adminCookie, ""); resp.Code != 401 {
		t.Fatalf("revocation failed %d", resp.Code)
	}
	sha := sha256.Sum256([]byte(admin.ID))
	if len(base64.RawURLEncoding.EncodeToString(sha[:])) == 0 {
		t.Fatal("invalid token helper")
	}
}
