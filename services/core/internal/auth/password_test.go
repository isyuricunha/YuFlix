package auth

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestPasswordHashAndVerify(t *testing.T) {
	a, err := HashPassword("A very secure password 123")
	if err != nil {
		t.Fatal(err)
	}
	b, err := HashPassword("A very secure password 123")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("salt reused")
	}
	if !VerifyPassword("A very secure password 123", a) {
		t.Fatal("verification failed")
	}
	if VerifyPassword("bad password", a) {
		t.Fatal("accepted invalid password")
	}
	if VerifyPassword(strings.Repeat("x", 129), a) {
		t.Fatal("accepted oversized password")
	}
	if VerifyPassword("A very secure password 123", "invalid") {
		t.Fatal("accepted invalid hash")
	}
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("accepted weak password")
	}
}
func TestRandomToken(t *testing.T) {
	x, raw, err := Token()
	if err != nil {
		t.Fatal(err)
	}
	y, _, err := Token()
	if err != nil {
		t.Fatal(err)
	}
	if x == y || len(raw) != 32 {
		t.Fatal("weak token")
	}
	v, err := base64.RawURLEncoding.DecodeString(x)
	if err != nil || len(v) != 32 {
		t.Fatal("invalid token encoding")
	}
}
