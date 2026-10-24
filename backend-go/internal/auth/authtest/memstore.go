// Package authtest provides an in-memory auth.Store for tests that run without PostgreSQL.
package authtest

import (
	"context"
	"sync"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/store"
)

type session struct {
	userID    int64
	method    string
	expiresAt time.Time
}

// MemStore keeps users and sessions in maps. Safe for concurrent use.
type MemStore struct {
	mu       sync.Mutex
	nextID   int64
	users    map[string]store.User // by username
	sessions map[string]session    // by string(tokenHash)
	Now      func() time.Time      // clock for expiry checks; time.Now by default
}

// NewMemStore returns an empty store.
func NewMemStore() *MemStore {
	return &MemStore{users: map[string]store.User{}, sessions: map[string]session{}, Now: time.Now}
}

func (m *MemStore) UpsertPasswordUser(_ context.Context, username, displayName, hash string) (store.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[username]
	if !ok {
		m.nextID++
		u = store.User{ID: m.nextID, Username: username}
	}
	u.DisplayName, u.PasswordHash = displayName, hash
	m.users[username] = u
	return u, nil
}

func (m *MemStore) UserByUsername(_ context.Context, username string) (store.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[username]
	if !ok {
		return store.User{}, store.ErrNotFound
	}
	return u, nil
}

func (m *MemStore) CreateSession(_ context.Context, hash []byte, userID int64, method string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[string(hash)] = session{userID: userID, method: method, expiresAt: expiresAt}
	return nil
}

func (m *MemStore) SessionUser(_ context.Context, hash []byte) (store.User, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[string(hash)]
	if !ok || !s.expiresAt.After(m.Now()) {
		return store.User{}, "", store.ErrNotFound
	}
	for _, u := range m.users {
		if u.ID == s.userID {
			return u, s.method, nil
		}
	}
	return store.User{}, "", store.ErrNotFound
}

func (m *MemStore) DeleteSession(_ context.Context, hash []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, string(hash))
	return nil
}

func (m *MemStore) DeleteExpiredSessions(_ context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int64
	for k, s := range m.sessions {
		if !s.expiresAt.After(m.Now()) {
			delete(m.sessions, k)
			n++
		}
	}
	return n, nil
}

// Sessions reports how many sessions are stored (live or expired).
func (m *MemStore) Sessions() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}
