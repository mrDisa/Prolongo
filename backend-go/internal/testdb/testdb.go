// Package testdb gives each integration test its own throwaway PostgreSQL database.
//
// Go runs the tests of different packages in parallel. If they shared one database,
// one test creating or truncating tables would break another. Here every test gets
// a fresh database "prolongo_t_<random>" next to TEST_DATABASE_URL, dropped afterwards.
// Without TEST_DATABASE_URL the test is skipped.
//
// Only test code imports this package.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New creates an empty database for this test and returns a pool connected to it.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()

	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("testdb: connect: %v", err)
	}
	name := "prolongo_t_" + randomHex(6)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close(ctx)
		t.Fatalf("testdb: create database: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("testdb: %v", err)
	}
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("testdb: open %s: %v", name, err)
	}

	t.Cleanup(func() {
		pool.Close()
		// FORCE disconnects anything still attached (PostgreSQL 13+).
		if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("testdb: drop %s: %v", name, err)
		}
		admin.Close(ctx)
	})
	return pool
}

// SourceSchema creates the source tables the way ingest does: the organizers'
// 001_schema.up.sql plus public.source_clock (Django migration 0002).
func SourceSchema(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()
	execFile(t, pool, "001_schema.up.sql")
	_, err := pool.Exec(context.Background(), `
		CREATE TABLE public.source_clock (
			id smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1),
			data_time timestamptz NOT NULL
		)`)
	if err != nil {
		t.Fatalf("testdb: create source_clock: %v", err)
	}
}

// Seed loads the organizers' reference data (002_seed.sql = data.json: 10 accounts,
// 30 services, 100 transactions). Call SourceSchema first.
func Seed(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()
	execFile(t, pool, "002_seed.sql")
}

// execFile runs one SQL file from data/test-data. Without arguments pgx uses the
// simple protocol, so a file with many statements (and its own BEGIN/COMMIT) works.
func execFile(t testing.TB, pool *pgxpool.Pool, name string) {
	t.Helper()
	sql, err := os.ReadFile(filepath.Join(testDataDir(), name))
	if err != nil {
		t.Fatalf("testdb: %v", err)
	}
	if _, err := pool.Exec(context.Background(), string(sql)); err != nil {
		t.Fatalf("testdb: %s: %v", name, err)
	}
}

// testDataDir is <repo>/data/test-data, found from this file's location,
// so it works whatever directory the test runs in.
func testDataDir() string {
	_, self, _, _ := runtime.Caller(0) // .../backend-go/internal/testdb/testdb.go
	return filepath.Join(filepath.Dir(self), "..", "..", "..", "data", "test-data")
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
