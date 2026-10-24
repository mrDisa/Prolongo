package domain_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrDisa/Prolongo/backend-go/internal/domain"
)

// The organizers' reference data, read from the repository (not copied).
const dataJSON = "../../../data/test-data/data.json"

func loadReference(t *testing.T) *domain.Dataset {
	t.Helper()
	f, err := os.Open(dataJSON)
	require.NoError(t, err)
	defer f.Close()
	ds, err := domain.FromJSON(f)
	require.NoError(t, err)
	return ds
}

func TestFromJSONReference(t *testing.T) {
	ds := loadReference(t)

	a, s, tx := ds.Counts()
	assert.Equal(t, [3]int{10, 30, 100}, [3]int{a, s, tx})
	assert.Equal(t, time.Date(2026, 10, 19, 14, 35, 53, 0, time.UTC), ds.LatestTimestamp)
	assert.Nil(t, ds.SourceTime, "data.json has no source clock")

	// Spot checks against docs/ОБЩЕЕ_ТЗ.md §4.2.1 (customer 5).
	acc := ds.Accounts[4]
	assert.Equal(t, domain.Account{ID: 5, Email: "user-0005@example.test", Balance: 28657, WithdrawBalance: 7623,
		TotalReplenished: 293608, BonusBalance: 9863, BonusUnlocked: false}, acc)

	svc := ds.ServicesByAccount[5]
	require.Len(t, svc, 3)
	assert.Equal(t, []int64{5, 15, 25}, []int64{svc[0].ID, svc[1].ID, svc[2].ID}, "ordered by id")
	s25 := svc[2]
	assert.Equal(t, domain.TermHour, s25.PaymentTerm)
	assert.Equal(t, domain.StatusProlongWait, s25.Status)
	assert.True(t, s25.AutoProlong)
	assert.EqualValues(t, 651, s25.Price)
	assert.Equal(t, time.Date(2025, 9, 14, 16, 0, 0, 0, time.UTC), s25.ExpiresAt)

	var tx75 *domain.Transaction
	for _, x := range ds.TxByService[15] {
		if x.ID == 75 {
			tx75 = &x
		}
	}
	require.NotNil(t, tx75)
	assert.False(t, tx75.Performed)
	assert.Nil(t, tx75.PerformedAt)
	assert.Equal(t, domain.TxProlong, tx75.Type)
}

func TestTermDays(t *testing.T) {
	cases := map[domain.PaymentTerm]float64{
		domain.TermHour: 1.0 / 24, domain.TermWeek: 7, domain.TermMonth: 30,
		domain.TermQuarterYear: 91, domain.TermHalfYear: 182, domain.TermYear: 365,
		"fortnight": 0,
	}
	for term, want := range cases {
		assert.InDelta(t, want, domain.TermDays(term), 1e-12, string(term))
	}
}

func TestLive(t *testing.T) {
	live := map[domain.ServiceStatus]bool{
		domain.StatusActive: true, domain.StatusSuspended: true, domain.StatusProlongWait: true,
		domain.StatusBlocked: false, domain.StatusDeleted: false,
	}
	for status, want := range live {
		assert.Equal(t, want, domain.Service{Status: status}.Live(), string(status))
	}
}

func TestBuilderRejectsUnknownValues(t *testing.T) {
	cases := map[string]string{
		"bonusState":  `{"accounts":[{"id":1,"email":"a","bonusState":"frozen"}],"services":[],"transactions":[]}`,
		"paymentTerm": `{"accounts":[],"services":[{"id":1,"ownerId":1,"paymentTerm":"fortnight","status":"active","createdAt":"2026-01-01T00:00:00Z","expiresAt":"2026-01-02T00:00:00Z"}],"transactions":[]}`,
		"status":      `{"accounts":[],"services":[{"id":1,"ownerId":1,"paymentTerm":"month","status":"paused","createdAt":"2026-01-01T00:00:00Z","expiresAt":"2026-01-02T00:00:00Z"}],"transactions":[]}`,
		"type":        `{"accounts":[],"services":[],"transactions":[{"id":1,"serviceId":1,"status":"performed","type":"refund","createdAt":"2026-01-01T00:00:00Z"}]}`,
	}
	for field, body := range cases {
		_, err := domain.FromJSON(strings.NewReader(body))
		assert.ErrorContains(t, err, field)
	}
}

func TestBuilderSortsByID(t *testing.T) {
	b := domain.NewBuilder()
	b.AddAccount(domain.Account{ID: 2}, "locked")
	b.AddAccount(domain.Account{ID: 1}, "unlocked")
	ds, err := b.Dataset()
	require.NoError(t, err)
	assert.Equal(t, int64(1), ds.Accounts[0].ID)
	assert.True(t, ds.Accounts[0].BonusUnlocked)
}
