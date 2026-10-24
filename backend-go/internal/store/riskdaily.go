package store

import (
	"context"
	"fmt"
	"time"
)

// RiskDaily is one day of core.risk_daily: the state of risk at the latest calculation of that day.
// The churn timeline (G11) shows it as the "services at risk" history.
type RiskDaily struct {
	Day                                          time.Time // the date matters, time of day is ignored
	CustomersHigh, CustomersMedium, CustomersLow int
	CustomersNone, ServicesAtRisk                int
	ExpectedLossCents                            int64
}

// UpsertRiskDaily saves the day's numbers; a later calculation of the same day replaces them.
func (s *Store) UpsertRiskDaily(ctx context.Context, d RiskDaily) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO core.risk_daily (day, customers_high, customers_medium, customers_low, customers_none,
		                             services_at_risk, expected_loss_cents, updated_at)
		VALUES ($1::date, $2, $3, $4, $5, $6, $7, now())
		ON CONFLICT (day) DO UPDATE SET
			customers_high = EXCLUDED.customers_high, customers_medium = EXCLUDED.customers_medium,
			customers_low = EXCLUDED.customers_low, customers_none = EXCLUDED.customers_none,
			services_at_risk = EXCLUDED.services_at_risk, expected_loss_cents = EXCLUDED.expected_loss_cents,
			updated_at = now()`,
		d.Day.UTC().Format(time.DateOnly), d.CustomersHigh, d.CustomersMedium, d.CustomersLow, d.CustomersNone,
		d.ServicesAtRisk, d.ExpectedLossCents,
	)
	if err != nil {
		return fmt.Errorf("store: upsert risk_daily: %w", err)
	}
	return nil
}
