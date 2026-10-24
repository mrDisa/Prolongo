// Package db opens the PostgreSQL pool and prepares the core schema.
package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"

	"github.com/mrDisa/Prolongo/backend-go/migrations"
)

// Open connects to PostgreSQL, retrying until ctx is done: in compose the
// database may still be starting when core starts.
func Open(ctx context.Context, url string, logger *slog.Logger) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("db: DATABASE_URL: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}
	err = retry(ctx, 2*time.Second, func() error { return pool.Ping(ctx) }, func(err error) {
		logger.Info("waiting for PostgreSQL", slog.Any("error", err))
	})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: PostgreSQL is not reachable: %w", err)
	}
	return pool, nil
}

// WaitForSource blocks until Django has created the source tables (public.services).
// core only reads them, so starting earlier would fail on the first recalculation.
func WaitForSource(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration, logger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err := retry(ctx, 2*time.Second, func() error {
		var name *string
		if err := pool.QueryRow(ctx, `SELECT to_regclass('public.services')::text`).Scan(&name); err != nil {
			return err
		}
		if name == nil {
			return errors.New("public.services does not exist yet")
		}
		return nil
	}, func(err error) {
		logger.Info("waiting for ingest to create the source schema", slog.Any("reason", err))
	})
	if err != nil {
		return fmt.Errorf("db: source schema did not appear within %s: %w", timeout, err)
	}
	return nil
}

// Migrate applies the embedded migrations of the core schema.
// Versions are tracked in core.goose_db_version, apart from Django's tables.
func Migrate(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	// goose creates its version table before running the first migration,
	// so the schema that holds that table must exist beforehand.
	if _, err := pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS core`); err != nil {
		return fmt.Errorf("db: create schema core: %w", err)
	}

	store, err := database.NewStore(database.DialectPostgres, "core.goose_db_version")
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()

	provider, err := goose.NewProvider("", sqlDB, migrations.FS, goose.WithStore(store))
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("db: migrate: %w", err)
	}
	for _, r := range results {
		logger.Info("migration applied", slog.String("file", r.Source.Path), slog.Duration("took", r.Duration))
	}
	return nil
}

// retry calls fn every interval until it succeeds or ctx is done.
func retry(ctx context.Context, interval time.Duration, fn func() error, onFail func(error)) error {
	for {
		err := fn()
		if err == nil {
			return nil
		}
		onFail(err)
		select {
		case <-ctx.Done():
			return errors.Join(ctx.Err(), err)
		case <-time.After(interval):
		}
	}
}
