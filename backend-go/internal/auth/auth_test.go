package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/auth"
	"github.com/mrDisa/Prolongo/backend-go/internal/auth/authtest"
	"github.com/mrDisa/Prolongo/backend-go/internal/store"
)

// Compile-time check: the real PostgreSQL store satisfies auth.Store.
var _ auth.Store = (*store.Store)(nil)

func newService(t *testing.T) (*auth.Service, *authtest.MemStore) {
	t.Helper()
	mem := authtest.NewMemStore()
	svc, err := auth.NewService(mem, time.Hour)
	require.NoError(t, err)
	require.NoError(t, svc.EnsureAdmin(context.Background(), "admin", "admin123"))
	return svc, mem
}

func TestLoginAuthenticateLogout(t *testing.T) {
	ctx := context.Background()
	svc, mem := newService(t)

	u, sess, err := svc.Login(ctx, "admin", "admin123")
	require.NoError(t, err)
	assert.Equal(t, "admin", u.Username)
	assert.Equal(t, auth.AdminDisplayName, u.DisplayName)
	assert.Equal(t, auth.MethodPassword, u.AuthMethod)
	assert.Len(t, sess.Token, 43, "32 bytes in unpadded base64url")
	assert.WithinDuration(t, time.Now().Add(time.Hour), sess.ExpiresAt, 5*time.Second)

	got, err := svc.Authenticate(ctx, sess.Token)
	require.NoError(t, err)
	assert.Equal(t, u, got)

	require.NoError(t, svc.Logout(ctx, sess.Token))
	_, err = svc.Authenticate(ctx, sess.Token)
	assert.ErrorIs(t, err, auth.ErrNoSession)
	assert.Zero(t, mem.Sessions())
}

func TestLoginRejectsWrongCredentials(t *testing.T) {
	ctx := context.Background()
	svc, mem := newService(t)

	for _, c := range []struct{ user, pass string }{
		{"admin", "wrong"},
		{"nobody", "admin123"},
		{"admin", ""},
	} {
		_, _, err := svc.Login(ctx, c.user, c.pass)
		assert.ErrorIs(t, err, auth.ErrInvalidCredentials, "%s / %s", c.user, c.pass)
	}
	assert.Zero(t, mem.Sessions(), "failed logins must not create sessions")
}

func TestEnsureAdminResetsPassword(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	require.NoError(t, svc.EnsureAdmin(ctx, "admin", "new-secret"))
	_, _, err := svc.Login(ctx, "admin", "admin123")
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
	_, _, err = svc.Login(ctx, "admin", "new-secret")
	assert.NoError(t, err)
}

func TestAuthenticateRejectsUnknownAndExpired(t *testing.T) {
	ctx := context.Background()
	svc, mem := newService(t)

	_, err := svc.Authenticate(ctx, "")
	assert.ErrorIs(t, err, auth.ErrNoSession)
	_, err = svc.Authenticate(ctx, "not-a-real-token")
	assert.ErrorIs(t, err, auth.ErrNoSession)

	_, sess, err := svc.Login(ctx, "admin", "admin123")
	require.NoError(t, err)
	mem.Now = func() time.Time { return time.Now().Add(2 * time.Hour) } // jump past the 1 h TTL
	_, err = svc.Authenticate(ctx, sess.Token)
	assert.ErrorIs(t, err, auth.ErrNoSession)
}

func TestTokensAreUnique(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	seen := map[string]bool{}
	for range 5 {
		_, sess, err := svc.Login(ctx, "admin", "admin123")
		require.NoError(t, err)
		assert.False(t, seen[sess.Token])
		seen[sess.Token] = true
	}
}

func TestUserContext(t *testing.T) {
	_, ok := auth.UserFrom(context.Background())
	assert.False(t, ok)

	want := auth.User{ID: 7, Username: "marketer"}
	got, ok := auth.UserFrom(auth.WithUser(context.Background(), want))
	assert.True(t, ok)
	assert.Equal(t, want, got)
}
