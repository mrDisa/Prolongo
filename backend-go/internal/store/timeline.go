package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mrDisa/Prolongo/backend-go/internal/engine"
)

// TimelineFacts reads the past of the churn chart for [from, to) (docs/ТЗ_BACKEND_GO.md §9.4):
// deletions per day, services with any recorded deletion, and services at risk per day.
// One read-only snapshot, so the three answers agree with each other.
func (s *Store) TimelineFacts(ctx context.Context, from, to time.Time) (engine.TimelineFacts, error) {
	facts := engine.TimelineFacts{
		Deletions: map[string]int{}, DeletedWithEvent: map[int64]bool{}, ServicesAtRisk: map[string]int{},
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return facts, fmt.Errorf("store: timeline: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // read-only: nothing to undo

	err = collect(ctx, tx, `
		SELECT to_char((occurred_at AT TIME ZONE 'UTC')::date, 'YYYY-MM-DD'), count(*)
		FROM public.status_events
		WHERE to_status = 'deleted' AND occurred_at >= $1 AND occurred_at < $2
		GROUP BY 1`,
		[]any{from, to}, func(rows pgx.Rows) error {
			var day string
			var n int
			err := rows.Scan(&day, &n)
			facts.Deletions[day] = n
			return err
		})
	if err != nil {
		return facts, fmt.Errorf("store: timeline deletions: %w", err)
	}

	err = collect(ctx, tx, `SELECT DISTINCT service_id FROM public.status_events WHERE to_status = 'deleted'`,
		nil, func(rows pgx.Rows) error {
			var id int64
			err := rows.Scan(&id)
			facts.DeletedWithEvent[id] = true
			return err
		})
	if err != nil {
		return facts, fmt.Errorf("store: timeline deleted services: %w", err)
	}

	err = collect(ctx, tx, `
		SELECT to_char(day, 'YYYY-MM-DD'), services_at_risk
		FROM core.risk_daily
		WHERE day >= $1::date AND day < $2::date`,
		[]any{from.UTC().Format(time.DateOnly), to.UTC().Format(time.DateOnly)}, func(rows pgx.Rows) error {
			var day string
			var n int
			err := rows.Scan(&day, &n)
			facts.ServicesAtRisk[day] = n
			return err
		})
	if err != nil {
		return facts, fmt.Errorf("store: timeline risk_daily: %w", err)
	}
	return facts, nil
}

// collect runs a query and calls scan for every row.
func collect(ctx context.Context, tx pgx.Tx, sql string, args []any, scan func(pgx.Rows) error) error {
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
