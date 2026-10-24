// Package store reads and writes the core schema in PostgreSQL.
package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound means the requested row does not exist.
var ErrNotFound = errors.New("not found")

// Store wraps the connection pool. One Store is shared by the whole app.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store on top of an open pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}
