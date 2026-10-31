package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	memoryKiB   = 32 * 1024
	iterations  = 3
	parallelism = 1
	keyLength   = 32
)

var ErrWeakPassword = errors.New("password must be 12 to 128 bytes long")

func ValidatePassword(password string) error {
	if len(password) < 12 || len(password) > 128 {
		return ErrWeakPassword
	}
	return nil
}
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, iterations, memoryKiB, parallelism, keyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memoryKiB, iterations, parallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func VerifyPassword(password, encoded string) bool {
	if len(password) > 128 {
		return false
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var m, t uint32
	var p uint8
	n, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p)
	if err != nil || n != 3 || m != memoryKiB || t != iterations || p != parallelism {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		return false
	}
	expect, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expect) != keyLength {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, t, m, p, keyLength)
	return subtle.ConstantTimeCompare(actual, expect) == 1
}

func Token() (string, []byte, error) {
	v := make([]byte, 32)
	if _, err := rand.Read(v); err != nil {
		return "", nil, err
	}
	return base64.RawURLEncoding.EncodeToString(v), v, nil
}

var usernamePattern = regexp.MustCompile("^[a-z][a-z0-9_]{2,31}$")

func ValidUsername(s string) bool { return usernamePattern.MatchString(s) }
