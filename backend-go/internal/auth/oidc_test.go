package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/auth"
)

func TestLoginOIDC(t *testing.T) {
	svc, mem := newService(t)
	ctx := context.Background()

	u, sess, err := svc.LoginOIDC(ctx, "sub-1", "marketer", "Маркетолог Prolongo")
	require.NoError(t, err)
	assert.Equal(t, auth.User{ID: u.ID, Username: "marketer", DisplayName: "Маркетолог Prolongo", AuthMethod: auth.MethodOIDC}, u)
	assert.NotEmpty(t, sess.Token)

	got, err := svc.Authenticate(ctx, sess.Token)
	require.NoError(t, err)
	assert.Equal(t, u, got, "the session remembers it came from SSO")

	again, _, err := svc.LoginOIDC(ctx, "sub-1", "marketer", "Маркетолог")
	require.NoError(t, err)
	assert.Equal(t, u.ID, again.ID, "the second sign-in finds the same user")
	assert.Equal(t, 2, mem.Sessions())

	_, _, err = svc.Login(ctx, "marketer", "")
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials, "no password: the form cannot open an SSO user")

	_, _, err = svc.LoginOIDC(ctx, "", "nobody", "")
	assert.Error(t, err, "an identity without subject is refused")
}
