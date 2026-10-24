package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
)

// LoadDataset reads public.accounts, services, transactions and source_clock
// (tables owned by Django; core only reads them).
//
// All queries run in one read-only REPEATABLE READ transaction: they see the
// database at the same instant even if the simulator writes in between, so a
// transaction never points to a service that "does not exist yet".
func (s *Store) LoadDataset(ctx context.Context) (*domain.Dataset, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("store: load dataset: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // read-only: nothing to undo, error does not matter

	b := domain.NewBuilder()
	steps := []struct {
		name string
		load func(context.Context, pgx.Tx, *domain.Builder) error
	}{
		{"accounts", loadAccounts},
		{"services", loadServices},
		{"transactions", loadTransactions},
		{"source_clock", loadSourceClock},
	}
	for _, step := range steps {
		if err := step.load(ctx, tx, b); err != nil {
			return nil, fmt.Errorf("store: load %s: %w", step.name, err)
		}
	}
	return b.Dataset()
}

func loadAccounts(ctx context.Context, tx pgx.Tx, b *domain.Builder) error {
	rows, err := tx.Query(ctx, `
		SELECT id, email, balance, "withdrawBalance", "totalReplenished", "bonusBalance", "bonusState"
		FROM public.accounts ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			a     domain.Account
			state string
		)
		if err := rows.Scan(&a.ID, &a.Email, &a.Balance, &a.WithdrawBalance, &a.TotalReplenished, &a.BonusBalance, &state); err != nil {
			return err
		}
		b.AddAccount(a, state)
	}
	return rows.Err()
}

func loadServices(ctx context.Context, tx pgx.Tx, b *domain.Builder) error {
	rows, err := tx.Query(ctx, `
		SELECT id, name, "ownerId", "paymentTerm", "autoProlong", "createdAt", "expiresAt", status, price
		FROM public.services ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			s            domain.Service
			term, status string
		)
		if err := rows.Scan(&s.ID, &s.Name, &s.OwnerID, &term, &s.AutoProlong, &s.CreatedAt, &s.ExpiresAt, &status, &s.Price); err != nil {
			return err
		}
		s.PaymentTerm, s.Status = domain.PaymentTerm(term), domain.ServiceStatus(status)
		s.CreatedAt, s.ExpiresAt = s.CreatedAt.UTC(), s.ExpiresAt.UTC()
		b.AddService(s)
	}
	return rows.Err()
}

func loadTransactions(ctx context.Context, tx pgx.Tx, b *domain.Builder) error {
	rows, err := tx.Query(ctx, `
		SELECT id, amount, status, "performedAt", "createdAt", type, "serviceId"
		FROM public.transactions ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			t            domain.Transaction
			status, kind string
		)
		if err := rows.Scan(&t.ID, &t.Amount, &status, &t.PerformedAt, &t.CreatedAt, &kind, &t.ServiceID); err != nil {
			return err
		}
		t.Type, t.CreatedAt = domain.TxType(kind), t.CreatedAt.UTC()
		if t.PerformedAt != nil {
			p := t.PerformedAt.UTC()
			t.PerformedAt = &p
		}
		b.AddTransaction(t, status)
	}
	return rows.Err()
}

// loadSourceClock reads the single row of public.source_clock; no row is fine (SourceTime stays nil).
func loadSourceClock(ctx context.Context, tx pgx.Tx, b *domain.Builder) error {
	var t time.Time
	err := tx.QueryRow(ctx, `SELECT data_time FROM public.source_clock WHERE id = 1`).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	b.SetSourceTime(t)
	return nil
}
