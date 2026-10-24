// Package auth implements password and OIDC login and server-side sessions.
// It knows nothing about HTTP: cookies and status codes live in internal/api.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/store"
)

// Auth methods stored in core.sessions.auth_method.
const (
	MethodPassword = "password"
	MethodOIDC     = "oidc"
)

// AdminDisplayName is shown in the panel header for the ADMIN_USERNAME user.
const AdminDisplayName = "Администратор"

var (
	// ErrInvalidCredentials: wrong username or password. Deliberately does not say which.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrNoSession: no token, unknown token or expired session.
	ErrNoSession = errors.New("no session")
)

// Store is what auth needs from the database. *store.Store implements it;
// tests use an in-memory fake (package authtest).
type Store interface {
	UpsertPasswordUser(ctx context.Context, username, displayName, passwordHash string) (store.User, error)
	UserByUsername(ctx context.Context, username string) (store.User, error)
	UpsertOIDCUser(ctx context.Context, subject, username, displayName string) (store.User, error)
	CreateSession(ctx context.Context, tokenHash []byte, userID int64, authMethod string, expiresAt time.Time) error
	SessionUser(ctx context.Context, tokenHash []byte) (store.User, string, error)
	DeleteSession(ctx context.Context, tokenHash []byte) error
	DeleteExpiredSessions(ctx context.Context) (int64, error)
}

// User is the signed-in user as the API shows it.
type User struct {
	ID          int64
	Username    string
	DisplayName string
	AuthMethod  string
}

// Session is a freshly created session: the token goes into the cookie.
type Session struct {
	Token     string
	ExpiresAt time.Time
}

// Service handles login, session checks and logout.
type Service struct {
	store     Store
	ttl       time.Duration
	now       func() time.Time
	dummyHash string
}

// NewService builds the service. ttl is how long a session lives (SESSION_TTL_HOURS).
func NewService(s Store, ttl time.Duration) (*Service, error) {
	// Compared against when the user does not exist, so that a wrong username
	// takes as long as a wrong password and response time leaks nothing.
	dummy, err := HashPassword("prolongo-dummy-password")
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	return &Service{store: s, ttl: ttl, now: time.Now, dummyHash: dummy}, nil
}

// EnsureAdmin creates the admin user or resets its password to the one from env.
func (s *Service) EnsureAdmin(ctx context.Context, username, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("auth: hash admin password: %w", err)
	}
	if _, err := s.store.UpsertPasswordUser(ctx, username, AdminDisplayName, hash); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	return nil
}

// Login checks the password and opens a new session.
func (s *Service) Login(ctx context.Context, username, password string) (User, Session, error) {
	u, err := s.store.UserByUsername(ctx, username)
	switch {
	case errors.Is(err, store.ErrNotFound):
		checkPassword(s.dummyHash, password) // same work as a real check, result ignored
		return User{}, Session{}, ErrInvalidCredentials
	case err != nil:
		return User{}, Session{}, fmt.Errorf("auth: login: %w", err)
	}
	if u.PasswordHash == "" || !checkPassword(u.PasswordHash, password) {
		return User{}, Session{}, ErrInvalidCredentials
	}

	sess, err := s.openSession(ctx, u.ID, MethodPassword)
	if err != nil {
		return User{}, Session{}, fmt.Errorf("auth: login: %w", err)
	}
	return toUser(u, MethodPassword), sess, nil
}

// LoginOIDC opens a session for a person the OIDC provider has vouched for (internal/oidc),
// creating the user on the first sign-in. The user has no password: only SSO lets them in.
func (s *Service) LoginOIDC(ctx context.Context, subject, username, displayName string) (User, Session, error) {
	if subject == "" {
		return User{}, Session{}, errors.New("auth: oidc login without subject")
	}
	u, err := s.store.UpsertOIDCUser(ctx, subject, username, displayName)
	if err != nil {
		return User{}, Session{}, fmt.Errorf("auth: oidc login: %w", err)
	}
	sess, err := s.openSession(ctx, u.ID, MethodOIDC)
	if err != nil {
		return User{}, Session{}, fmt.Errorf("auth: oidc login: %w", err)
	}
	return toUser(u, MethodOIDC), sess, nil
}

func (s *Service) openSession(ctx context.Context, userID int64, method string) (Session, error) {
	token, hash := newToken()
	expires := s.now().Add(s.ttl)
	if err := s.store.CreateSession(ctx, hash, userID, method, expires); err != nil {
		return Session{}, err
	}
	return Session{Token: token, ExpiresAt: expires}, nil
}

// Authenticate returns the user of a session token, or ErrNoSession.
func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrNoSession
	}
	u, method, err := s.store.SessionUser(ctx, hashToken(token))
	switch {
	case errors.Is(err, store.ErrNotFound):
		return User{}, ErrNoSession
	case err != nil:
		return User{}, fmt.Errorf("auth: authenticate: %w", err)
	}
	return toUser(u, method), nil
}

// Logout deletes the session of the token.
func (s *Service) Logout(ctx context.Context, token string) error {
	if err := s.store.DeleteSession(ctx, hashToken(token)); err != nil {
		return fmt.Errorf("auth: logout: %w", err)
	}
	return nil
}

// CleanupLoop deletes expired sessions every interval until ctx is cancelled.
// Run it in its own goroutine.
func (s *Service) CleanupLoop(ctx context.Context, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := s.store.DeleteExpiredSessions(ctx)
			if err != nil {
				logger.Error("session cleanup failed", slog.Any("error", err))
				continue
			}
			if n > 0 {
				logger.Info("expired sessions removed", slog.Int64("count", n))
			}
		}
	}
}

func toUser(u store.User, method string) User {
	return User{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, AuthMethod: method}
}

type userKey struct{}

// WithUser stores the signed-in user in the request context.
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userKey{}, u)
}

// UserFrom returns the user put into the context by the session middleware.
func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userKey{}).(User)
	return u, ok
}
