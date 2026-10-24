package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// User is a panel user from core.users.
type User struct {
	ID           int64
	Username     string
	DisplayName  string
	PasswordHash string // empty for OIDC-only users
}

// UpsertPasswordUser creates the user or, if it exists, updates its display name and password hash.
// Used at startup for ADMIN_USERNAME / ADMIN_PASSWORD.
func (s *Store) UpsertPasswordUser(ctx context.Context, username, displayName, passwordHash string) (User, error) {
	u := User{Username: username, DisplayName: displayName, PasswordHash: passwordHash}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO core.users (username, display_name, password_hash)
		VALUES ($1, $2, $3)
		ON CONFLICT (username) DO UPDATE
		SET display_name = EXCLUDED.display_name, password_hash = EXCLUDED.password_hash
		RETURNING id`,
		username, displayName, passwordHash,
	).Scan(&u.ID)
	if err != nil {
		return User{}, fmt.Errorf("store: upsert user %q: %w", username, err)
	}
	return u, nil
}

// UserByUsername returns ErrNotFound if there is no such user.
func (s *Store) UserByUsername(ctx context.Context, username string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, username, display_name, COALESCE(password_hash, '')
		FROM core.users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("store: user %q: %w", username, err)
	}
	return u, nil
}
