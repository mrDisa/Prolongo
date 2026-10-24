package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/store"
)

func TestUpsertOIDCUser(t *testing.T) {
	s, _ := setup(t) // a fresh database per test: names do not collide
	ctx := context.Background()

	first, err := s.UpsertOIDCUser(ctx, "sub-marketer-1", "marketer", "Маркетолог")
	require.NoError(t, err)
	assert.Equal(t, "marketer", first.Username)

	again, err := s.UpsertOIDCUser(ctx, "sub-marketer-1", "renamed-at-provider", "Маркетолог Prolongo")
	require.NoError(t, err)
	assert.Equal(t, store.User{ID: first.ID, Username: "marketer", DisplayName: "Маркетолог Prolongo"}, again,
		"same subject → same user; the display name follows the provider, the username stays")

	got, err := s.UserByUsername(ctx, "marketer")
	require.NoError(t, err)
	assert.Empty(t, got.PasswordHash, "an SSO user has no password: the password form cannot open it")
}

// Someone named "admin" at the provider must not take over our password admin.
func TestUpsertOIDCUserNeverTakesAPasswordUser(t *testing.T) {
	s, _ := setup(t)
	ctx := context.Background()
	admin, err := s.UpsertPasswordUser(ctx, "admin", "Администратор", "hash")
	require.NoError(t, err)

	sso, err := s.UpsertOIDCUser(ctx, "a1b2c3d4e5f6", "admin", "Чужой admin")
	require.NoError(t, err)
	assert.NotEqual(t, admin.ID, sso.ID)
	assert.Equal(t, "admin-a1b2c3d4", sso.Username)

	got, err := s.UserByUsername(ctx, "admin")
	require.NoError(t, err)
	assert.Equal(t, store.User{ID: admin.ID, Username: "admin", DisplayName: "Администратор", PasswordHash: "hash"}, got, "untouched")

	back, err := s.UpsertOIDCUser(ctx, "a1b2c3d4e5f6", "admin", "Чужой admin")
	require.NoError(t, err)
	assert.Equal(t, sso.ID, back.ID, "the next sign-in finds the suffixed user by subject")
}
