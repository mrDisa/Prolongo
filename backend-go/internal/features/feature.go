// Package features holds the risk features of the scoring model.
//
// Each feature is one file: it describes itself (Meta), computes a value 0..1
// with a reason text (Compute) and registers itself in init(). The engine, the
// settings API and the UI pick up every registered feature automatically —
// adding a feature means adding a file (docs/ADD_FEATURE.md).
package features

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
)

// Scope says what a feature is computed for.
type Scope string

const (
	// ScopeService features are computed for every live service.
	ScopeService Scope = "service"
	// ScopeAccount features are computed once per account and added to each of its services.
	ScopeAccount Scope = "account"
)

// Meta describes a feature for the engine, the settings API and the UI.
type Meta struct {
	Key           string
	Title         string // Russian, for the UI
	Description   string // Russian, 1–2 sentences
	Scope         Scope
	DefaultWeight float64
	Min, Max      float64 // allowed weight range for the UI slider
	Action        string  // Russian recommendation for the marketer; empty = none
}

// Context is everything a feature may need about one account. The engine builds it once per account.
type Context struct {
	Now         time.Time
	HorizonDays float64
	Account     domain.Account
	Services    []domain.Service     // all services of the account
	Live        []domain.Service     // only live ones (active, suspended, prolong_wait)
	Tx          []domain.Transaction // all transactions of the account's services
	NeedH       int64                // Σ price × chargesH over live services: what the account must pay within the horizon
	Funds       int64                // balance + bonus balance if bonuses are unlocked
	MRR         int64                // Σ MRR over live services
}

// Feature is one risk signal.
type Feature interface {
	Meta() Meta
	// Compute returns a value in 0..1 and a reason text. For ScopeAccount, s is nil.
	// Value 0 means the feature does not apply and its reason is not shown.
	Compute(c *Context, s *domain.Service) (value float64, reason string)
}

// registry is the only package-level mutable state in core (allowed by the spec):
// it is filled once by init() functions before main starts and only read afterwards.
var registry = map[string]Feature{}

// Register adds a feature. Called from init(); a duplicate key is a programming error.
func Register(f Feature) {
	key := f.Meta().Key
	if _, dup := registry[key]; dup {
		panic(fmt.Sprintf("features: %q registered twice", key))
	}
	registry[key] = f
}

// All returns every registered feature in a stable order (by key).
func All() []Feature {
	all := make([]Feature, 0, len(registry))
	for _, f := range registry {
		all = append(all, f)
	}
	slices.SortFunc(all, func(a, b Feature) int { return cmp.Compare(a.Meta().Key, b.Meta().Key) })
	return all
}

// ByKey returns a registered feature.
func ByKey(key string) (Feature, bool) {
	f, ok := registry[key]
	return f, ok
}
