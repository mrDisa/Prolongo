package auth

import (
	"golang.org/x/crypto/bcrypt"
)

// bcryptCost 10 is about 50–80 ms per check: slow enough to resist brute force,
// fast enough for a login form.
const bcryptCost = 10

// HashPassword returns a bcrypt hash for storing in core.users.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// checkPassword reports whether password matches the bcrypt hash.
func checkPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
