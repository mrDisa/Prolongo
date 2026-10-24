// Package domain holds the source data model: accounts, services, transactions.
// It mirrors the organizers' schema (public.*) and knows nothing about storage.
package domain

import (
	"cmp"
	"fmt"
	"slices"
	"time"
)

// PaymentTerm is how often a service is paid for.
type PaymentTerm string

const (
	TermHour        PaymentTerm = "hour"
	TermWeek        PaymentTerm = "week"
	TermMonth       PaymentTerm = "month"
	TermQuarterYear PaymentTerm = "quarter_year"
	TermHalfYear    PaymentTerm = "half_year"
	TermYear        PaymentTerm = "year"
)

// termDays is the length of one paid period in days (docs/ОБЩЕЕ_ТЗ.md §4.1).
var termDays = map[PaymentTerm]float64{
	TermHour:        1.0 / 24,
	TermWeek:        7,
	TermMonth:       30,
	TermQuarterYear: 91,
	TermHalfYear:    182,
	TermYear:        365,
}

// TermDays returns the length of one period in days, or 0 for an unknown term.
func TermDays(t PaymentTerm) float64 {
	return termDays[t]
}

// TermDuration returns the length of one period as an exact duration, or 0 for an unknown term.
// Use it for counting whole periods: integer nanoseconds have no rounding error,
// while 30 / (1/24) in float64 may come out as 720.0000001 and round up to 721.
func TermDuration(t PaymentTerm) time.Duration {
	if t == TermHour {
		return time.Hour
	}
	return time.Duration(termDays[t]) * 24 * time.Hour
}

// ServiceStatus is the lifecycle state of a service.
type ServiceStatus string

const (
	StatusActive      ServiceStatus = "active"
	StatusBlocked     ServiceStatus = "blocked"
	StatusDeleted     ServiceStatus = "deleted"
	StatusSuspended   ServiceStatus = "suspended"
	StatusProlongWait ServiceStatus = "prolong_wait"
)

// TxType is a transaction kind.
type TxType string

const (
	TxOrder   TxType = "order"
	TxProlong TxType = "prolong"
)

// Account is a customer (public.accounts). Money is in euro cents.
type Account struct {
	ID               int64
	Email            string
	Balance          int64
	WithdrawBalance  int64
	TotalReplenished int64
	BonusBalance     int64
	BonusUnlocked    bool // bonusState == "unlocked": bonuses can pay for services
}

// Service is a paid service of an account (public.services).
type Service struct {
	ID          int64
	Name        string
	OwnerID     int64
	PaymentTerm PaymentTerm
	AutoProlong bool
	CreatedAt   time.Time
	ExpiresAt   time.Time
	Status      ServiceStatus
	Price       int64 // cents per period
}

// GraceDays: a prolong_wait service is deleted this many days after its paid period ends.
const GraceDays = 5

// DeletionDue is when the hoster deletes the service if it stays unpaid: ExpiresAt + GraceDays.
func (s Service) DeletionDue() time.Time {
	return s.ExpiresAt.AddDate(0, 0, GraceDays)
}

// Live reports whether the service still brings money: active, suspended or prolong_wait.
// blocked and deleted services are out of the risk model.
func (s Service) Live() bool {
	switch s.Status {
	case StatusActive, StatusSuspended, StatusProlongWait:
		return true
	}
	return false
}

// Transaction is a payment for a service (public.transactions).
type Transaction struct {
	ID          int64
	ServiceID   int64
	Amount      int64
	Performed   bool       // status == "performed"
	PerformedAt *time.Time // nil when the payment did not go through
	CreatedAt   time.Time
	Type        TxType
}

// Dataset is everything the risk engine reads, loaded at one consistent moment.
type Dataset struct {
	Accounts          []Account               // ordered by id
	ServicesByAccount map[int64][]Service     // services of each account, ordered by id
	TxByService       map[int64][]Transaction // transactions of each service, ordered by id

	// LatestTimestamp is max(services.createdAt, transactions.createdAt, transactions.performedAt):
	// the fallback "now" for clock mode data when there is no source clock.
	LatestTimestamp time.Time
	// SourceTime is public.source_clock.data_time, set by ingest and moved by the simulator; nil if absent.
	SourceTime *time.Time
}

// Counts returns how many accounts, services and transactions the dataset has.
func (d *Dataset) Counts() (accounts, services, transactions int) {
	for _, s := range d.ServicesByAccount {
		services += len(s)
	}
	for _, t := range d.TxByService {
		transactions += len(t)
	}
	return len(d.Accounts), services, transactions
}

// Builder assembles a Dataset row by row and validates enum values on the way.
// Both the PostgreSQL loader and FromJSON use it, so they produce identical datasets.
type Builder struct {
	ds  Dataset
	err error
}

// NewBuilder returns an empty builder.
func NewBuilder() *Builder {
	return &Builder{ds: Dataset{
		ServicesByAccount: map[int64][]Service{},
		TxByService:       map[int64][]Transaction{},
	}}
}

// AddAccount adds an account; bonusState must be "locked" or "unlocked".
func (b *Builder) AddAccount(a Account, bonusState string) {
	switch bonusState {
	case "unlocked":
		a.BonusUnlocked = true
	case "locked":
	default:
		b.fail(fmt.Errorf("account %d: unknown bonusState %q", a.ID, bonusState))
	}
	b.ds.Accounts = append(b.ds.Accounts, a)
}

// AddService adds a service, checking its status and payment term.
func (b *Builder) AddService(s Service) {
	if _, ok := termDays[s.PaymentTerm]; !ok {
		b.fail(fmt.Errorf("service %d: unknown paymentTerm %q", s.ID, s.PaymentTerm))
	}
	switch s.Status {
	case StatusActive, StatusBlocked, StatusDeleted, StatusSuspended, StatusProlongWait:
	default:
		b.fail(fmt.Errorf("service %d: unknown status %q", s.ID, s.Status))
	}
	b.ds.ServicesByAccount[s.OwnerID] = append(b.ds.ServicesByAccount[s.OwnerID], s)
	b.seen(s.CreatedAt)
}

// AddTransaction adds a transaction; status must be "performed" or "not_performed".
func (b *Builder) AddTransaction(t Transaction, status string) {
	switch status {
	case "performed":
		t.Performed = true
	case "not_performed":
	default:
		b.fail(fmt.Errorf("transaction %d: unknown status %q", t.ID, status))
	}
	if t.Type != TxOrder && t.Type != TxProlong {
		b.fail(fmt.Errorf("transaction %d: unknown type %q", t.ID, t.Type))
	}
	b.ds.TxByService[t.ServiceID] = append(b.ds.TxByService[t.ServiceID], t)
	b.seen(t.CreatedAt)
	if t.PerformedAt != nil {
		b.seen(*t.PerformedAt)
	}
}

// SetSourceTime records public.source_clock.data_time.
func (b *Builder) SetSourceTime(t time.Time) {
	t = t.UTC()
	b.ds.SourceTime = &t
}

// Dataset returns the result or the first validation error.
func (b *Builder) Dataset() (*Dataset, error) {
	if b.err != nil {
		return nil, b.err
	}
	ds := b.ds
	// Sort by id so the result does not depend on the order rows arrived in.
	slices.SortFunc(ds.Accounts, func(x, y Account) int { return cmp.Compare(x.ID, y.ID) })
	for _, s := range ds.ServicesByAccount {
		slices.SortFunc(s, func(x, y Service) int { return cmp.Compare(x.ID, y.ID) })
	}
	for _, t := range ds.TxByService {
		slices.SortFunc(t, func(x, y Transaction) int { return cmp.Compare(x.ID, y.ID) })
	}
	return &ds, nil
}

func (b *Builder) seen(t time.Time) {
	if t.After(b.ds.LatestTimestamp) {
		b.ds.LatestTimestamp = t.UTC()
	}
}

func (b *Builder) fail(err error) {
	if b.err == nil {
		b.err = fmt.Errorf("domain: %w", err)
	}
}
