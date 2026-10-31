package catalog

import (
	"context"
	"net/netip"
	"os"
	"testing"
	"time"
)

func TestPublicAddressValidation(t *testing.T) {
	if AllowedIP(netip.AddrFrom4([4]byte{192, 168, 1, 1})) {
		t.Fatal("RFC1918 address accepted")
	}

	for _, addr := range []string{
		"127.0.0.1", "::1", "10.12.0.1", "172.19.1.1",
		"100.100.100.100", "169.254.169.254", "198.18.0.1", "::ffff:127.0.0.1",
		"fc00::1", "2001:db8::1", "2002:c0a8:0101::1", "64:ff9b::c0a8:0101", "224.0.0.1", "203.0.113.10",
	} {
		if AllowedIP(netip.MustParseAddr(addr)) {
			t.Errorf("private/special IP allowed: %s", addr)
		}
	}
	for _, addr := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"} {
		if !AllowedIP(netip.MustParseAddr(addr)) {
			t.Errorf("public IP rejected: %s", addr)
		}
	}
}
func TestManifestURLSafety(t *testing.T) {
	good := "https://v3-cinemeta.strem.io/manifest.json"
	if got, err := ValidateManifestURL(good); err != nil || got != good {
		t.Fatalf("public HTTPS manifest rejected: %v", err)
	}
	for _, v := range []string{
		"http://v3-cinemeta.strem.io/manifest.json", "https://127.0.0.1/manifest.json",
		"https://example.com:443/manifest.json", "https://user:secret@example.com/manifest.json",
		"https://example.com/manifest.json?token=abc",
		"https://example.com/manifest.json#fragment",
		"https://example.com/key/manifest.json", "https://example.com/%6danifest.json",
		"https://example.com/manifest.json\nX: 1",
		"https://example.com\\@127.0.0.1/manifest.json",
		"https://localhost/manifest.json",
	} {
		if _, err := ValidateManifestURL(v); err == nil {
			t.Errorf("unsafe URL accepted: %q", v)
		}
	}
}
func TestCatalogFiltering(t *testing.T) {
	cats := []Catalog{
		{Type: "movie", ID: "top", Name: "Popular"},
		{Type: "series", ID: "top", Name: "Popular"},
		{Type: "movie", ID: "top", Name: "Duplicate"},
		{Type: "tv", ID: "iptv", Name: "Unsupported"},
		{Type: "movie", ID: "bad/id", Name: "Wrong ID"},
	}
	out := normalizeCatalogs(cats)
	if len(out) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(out))
	}
}
func TestArtworkURLSafety(t *testing.T) {
	if safeImage("https://images.metahub.space/poster/small/tt123/img") == "" {
		t.Fatal("valid URL rejected")
	}
	for _, v := range []string{
		"http://images.metahub.space/a", "https://127.0.0.1/a",
		"https://example.com/image.jpg?token=private",
		"javascript:alert(1)", "data:image/svg+xml,evil", "https://user:pass@example.com/a",
	} {
		if safeImage(v) != "" {
			t.Fatalf("unsafe image URL accepted: %s", v)
		}
	}
}
func TestPublicCinemetaIntegration(t *testing.T) {
	if os.Getenv("YUFLIX_TEST_PUBLIC_ADDON") != "true" {
		t.Skip("opt-in live upstream check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client := NewClient()
	m, err := client.Manifest(ctx, "https://v3-cinemeta.strem.io/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "Cinemeta" || len(m.Catalogs) == 0 {
		t.Fatalf("unexpected manifest metadata: %+v", m)
	}
	items, err := client.Catalog(ctx, "https://v3-cinemeta.strem.io/manifest.json", "movie", "top")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 || items[0].Name == "" || items[0].Poster == "" {
		t.Fatalf("catalog missing artwork: %+v", items)
	}
	meta, err := client.Meta(ctx, "https://v3-cinemeta.strem.io/manifest.json", "movie", items[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Name == "" || meta.ID != items[0].ID {
		t.Fatalf("invalid movie detail: %+v", meta)
	}

}
