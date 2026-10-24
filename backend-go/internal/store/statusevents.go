package store

import (
	"context"
	"fmt"
	"time"
)

// StatusEvent is one service status change from public.status_events (written by the ingest trigger).
type StatusEvent struct {
	ID         int64
	ServiceID  int64
	AccountID  int64
	FromStatus *string // nil when the service appeared
	ToStatus   string
	OccurredAt time.Time
	Source     string
}

// StatusHistory returns the latest status changes of an account's services, newest first.
func (s *Store) StatusHistory(ctx context.Context, accountID int64, limit int) ([]StatusEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, service_id, account_id, from_status, to_status, occurred_at, source
		FROM public.status_events
		WHERE account_id = $1
		ORDER BY occurred_at DESC, id DESC
		LIMIT $2`, accountID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: status history: %w", err)
	}
	defer rows.Close()
	events := []StatusEvent{}
	for rows.Next() {
		var e StatusEvent
		if err := rows.Scan(&e.ID, &e.ServiceID, &e.AccountID, &e.FromStatus, &e.ToStatus, &e.OccurredAt, &e.Source); err != nil {
			return nil, fmt.Errorf("store: status history: %w", err)
		}
		e.OccurredAt = e.OccurredAt.UTC()
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: status history: %w", err)
	}
	return events, nil
}
