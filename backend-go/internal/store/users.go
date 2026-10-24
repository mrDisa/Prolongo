package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// uniqueViolation is the PostgreSQL error code of a broken UNIQUE constraint.
const uniqueViolation = "23505"

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

// UpsertOIDCUser finds the user of an OIDC subject or creates it; the display name follows the
// provider. The user is matched by subject only, never by username: someone who calls
// themselves "admin" at the provider must not become our password admin. If the username is
// taken, a suffix from the subject is added. The username stays as created even if it changes
// at the provider.
func (s *Store) UpsertOIDCUser(ctx context.Context, subject, username, displayName string) (User, error) {
	suffix := subject
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	var u User
	var err error
	for _, name := range []string{username, username + "-" + suffix} {
		err = s.pool.QueryRow(ctx, `
			INSERT INTO core.users (username, display_name, oidc_subject)
			VALUES ($1, $2, $3)
			ON CONFLICT (oidc_subject) DO UPDATE SET display_name = EXCLUDED.display_name
			RETURNING id, username, display_name`,
			name, displayName, subject,
		).Scan(&u.ID, &u.Username, &u.DisplayName)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == "users_username_key" {
			continue // the name belongs to someone else: try with the suffix
		}
		break
	}
	if err != nil {
		return User{}, fmt.Errorf("store: upsert oidc user %q: %w", username, err)
	}
	return u, nil
}
