package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// CreateSession stores a session. Only the sha256 of the token is kept in the database.
func (s *Store) CreateSession(ctx context.Context, tokenHash []byte, userID int64, authMethod string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO core.sessions (token_hash, user_id, auth_method, expires_at)
		VALUES ($1, $2, $3, $4)`,
		tokenHash, userID, authMethod, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("store: create session: %w", err)
	}
	return nil
}

// SessionUser returns the user of a live (not expired) session and how they logged in.
// Returns ErrNotFound for unknown or expired tokens.
func (s *Store) SessionUser(ctx context.Context, tokenHash []byte) (User, string, error) {
	var (
		u      User
		method string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.display_name, s.auth_method
		FROM core.sessions s
		JOIN core.users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()`,
		tokenHash,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &method)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	if err != nil {
		return User{}, "", fmt.Errorf("store: session: %w", err)
	}
	return u, method, nil
}

// DeleteSession removes one session (logout). Deleting a missing session is not an error.
func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM core.sessions WHERE token_hash = $1`, tokenHash); err != nil {
		return fmt.Errorf("store: delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes expired sessions and reports how many were removed.
func (s *Store) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM core.sessions WHERE expires_at <= now()`)
	if err != nil {
		return 0, fmt.Errorf("store: delete expired sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}
