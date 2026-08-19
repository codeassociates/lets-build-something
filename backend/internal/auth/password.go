// Package auth handles identity: password storage, sessions, and the
// role checks the rest of the API is built on.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// PBKDF2-HMAC-SHA256 at the OWASP-recommended work factor. Stdlib as of Go 1.24,
// which keeps a password-hashing dependency out of the build.
const (
	pbkdf2Iterations = 600_000
	pbkdf2KeyLength  = 32
	saltLength       = 16
)

var ErrBadCredentials = errors.New("email or password is incorrect")

// HashPassword returns a self-describing hash: the parameters travel with the
// digest, so the work factor can be raised later without invalidating old rows.
func HashPassword(plaintext string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, plaintext, salt, pbkdf2Iterations, pbkdf2KeyLength)
	if err != nil {
		return "", fmt.Errorf("deriving key: %w", err)
	}
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s",
		pbkdf2Iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword compares in constant time. A malformed stored hash is a
// verification failure, never a panic.
func VerifyPassword(encoded, plaintext string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return ErrBadCredentials
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < 1 {
		return ErrBadCredentials
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return ErrBadCredentials
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return ErrBadCredentials
	}
	got, err := pbkdf2.Key(sha256.New, plaintext, salt, iterations, len(want))
	if err != nil {
		return ErrBadCredentials
	}
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrBadCredentials
	}
	return nil
}

// newSessionToken returns the secret handed to the browser and the digest
// stored in the database. The plaintext token is never persisted, so a dump of
// the sessions table cannot be replayed.
func newSessionToken() (token, digest string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generating session token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// GenerateTemporaryPassword makes a readable one-time password for staff to
// hand to a walk-in customer. The alphabet omits characters that are easily
// confused when read aloud or written down.
func GenerateTemporaryPassword() string {
	const alphabet = "abcdefghjkmnpqrstuvwxyz23456789"
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand does not fail in practice; refusing to return a weak
		// password is better than returning a predictable one.
		panic("could not generate a password: " + err.Error())
	}
	out := make([]byte, len(b))
	for i, v := range b {
		out[i] = alphabet[int(v)%len(alphabet)]
	}
	return string(out[:4]) + "-" + string(out[4:8]) + "-" + string(out[8:])
}
