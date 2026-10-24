package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// newToken returns a random session token for the cookie and its sha256 for the database.
// 32 random bytes = 256 bits: guessing a live token is not feasible.
func newToken() (token string, hash []byte) {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error since Go 1.24
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token)
}

// hashToken is what we store and look up: a leaked database does not reveal live tokens.
func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
