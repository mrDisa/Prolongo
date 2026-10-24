package store_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/db"
	"github.com/mrDisa/Prolongo/backend-go/internal/store"
	"github.com/mrDisa/Prolongo/backend-go/internal/testdb"
)

// Integration tests on a throwaway database (internal/testdb); skipped without TEST_DATABASE_URL.
func setup(t *testing.T) (*store.Store, *pgxpool.Pool) {
	t.Helper()
	pool := testdb.New(t)
	quiet := slog.New(slog.NewJSONHandler(io.Discard, nil))
	require.NoError(t, db.Migrate(context.Background(), pool, quiet))
	return store.New(pool), pool
}

// uniqueName keeps parallel runs and a shared dev database from colliding.
func uniqueName(t *testing.T) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "test-" + hex.EncodeToString(b)
}

func randomHash() []byte {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return b
}

func TestUsers(t *testing.T) {
	s, pool := setup(t)
	ctx := context.Background()
	name := uniqueName(t)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM core.users WHERE username = $1`, name) })

	_, err := s.UserByUsername(ctx, name)
	assert.ErrorIs(t, err, store.ErrNotFound)

	created, err := s.UpsertPasswordUser(ctx, name, "First", "hash-1")
	require.NoError(t, err)
	assert.NotZero(t, created.ID)

	updated, err := s.UpsertPasswordUser(ctx, name, "Second", "hash-2")
	require.NoError(t, err)
	assert.Equal(t, created.ID, updated.ID, "upsert keeps the id")

	got, err := s.UserByUsername(ctx, name)
	require.NoError(t, err)
	assert.Equal(t, store.User{ID: created.ID, Username: name, DisplayName: "Second", PasswordHash: "hash-2"}, got)
}

func TestSessions(t *testing.T) {
	s, pool := setup(t)
	ctx := context.Background()
	name := uniqueName(t)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM core.users WHERE username = $1`, name) })

	u, err := s.UpsertPasswordUser(ctx, name, "Tester", "hash")
	require.NoError(t, err)

	live, expired := randomHash(), randomHash()
	require.NoError(t, s.CreateSession(ctx, live, u.ID, "password", time.Now().Add(time.Hour)))
	require.NoError(t, s.CreateSession(ctx, expired, u.ID, "password", time.Now().Add(-time.Minute)))

	got, method, err := s.SessionUser(ctx, live)
	require.NoError(t, err)
	assert.Equal(t, u.ID, got.ID)
	assert.Equal(t, "Tester", got.DisplayName)
	assert.Equal(t, "password", method)

	_, _, err = s.SessionUser(ctx, expired)
	assert.ErrorIs(t, err, store.ErrNotFound, "expired sessions are invisible")
	_, _, err = s.SessionUser(ctx, randomHash())
	assert.ErrorIs(t, err, store.ErrNotFound)

	n, err := s.DeleteExpiredSessions(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, int64(1))

	require.NoError(t, s.DeleteSession(ctx, live))
	_, _, err = s.SessionUser(ctx, live)
	assert.ErrorIs(t, err, store.ErrNotFound)
	assert.NoError(t, s.DeleteSession(ctx, live), "deleting twice is fine")
}

func TestSessionsDisappearWithUser(t *testing.T) {
	s, pool := setup(t)
	ctx := context.Background()
	u, err := s.UpsertPasswordUser(ctx, uniqueName(t), "Tester", "hash")
	require.NoError(t, err)
	h := randomHash()
	require.NoError(t, s.CreateSession(ctx, h, u.ID, "password", time.Now().Add(time.Hour)))

	_, err = pool.Exec(ctx, `DELETE FROM core.users WHERE id = $1`, u.ID)
	require.NoError(t, err)
	_, _, err = s.SessionUser(ctx, h)
	assert.ErrorIs(t, err, store.ErrNotFound, "ON DELETE CASCADE removes the sessions")
}
