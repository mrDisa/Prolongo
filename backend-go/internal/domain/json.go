package domain

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// The organizers' data.json format (camelCase, money in cents).
type jsonFile struct {
	Accounts []struct {
		ID               int64  `json:"id"`
		Email            string `json:"email"`
		Balance          int64  `json:"balance"`
		WithdrawBalance  int64  `json:"withdrawBalance"`
		TotalReplenished int64  `json:"totalReplenished"`
		BonusBalance     int64  `json:"bonusBalance"`
		BonusState       string `json:"bonusState"`
	} `json:"accounts"`
	Services []struct {
		ID          int64     `json:"id"`
		Name        string    `json:"name"`
		OwnerID     int64     `json:"ownerId"`
		PaymentTerm string    `json:"paymentTerm"`
		AutoProlong bool      `json:"autoProlong"`
		CreatedAt   time.Time `json:"createdAt"`
		ExpiresAt   time.Time `json:"expiresAt"`
		Status      string    `json:"status"`
		Price       int64     `json:"price"`
	} `json:"services"`
	Transactions []struct {
		ID          int64      `json:"id"`
		Amount      int64      `json:"amount"`
		Status      string     `json:"status"`
		PerformedAt *time.Time `json:"performedAt"`
		CreatedAt   time.Time  `json:"createdAt"`
		Type        string     `json:"type"`
		ServiceID   int64      `json:"serviceId"`
	} `json:"transactions"`
}

// FromJSON builds a Dataset from a data.json file of the organizers.
// The risk engine tests use it to run on the reference data without PostgreSQL.
// The file has no source clock, so SourceTime stays nil.
func FromJSON(r io.Reader) (*Dataset, error) {
	var f jsonFile
	if err := json.NewDecoder(r).Decode(&f); err != nil {
		return nil, fmt.Errorf("domain: data.json: %w", err)
	}
	b := NewBuilder()
	for _, a := range f.Accounts {
		b.AddAccount(Account{
			ID: a.ID, Email: a.Email, Balance: a.Balance, WithdrawBalance: a.WithdrawBalance,
			TotalReplenished: a.TotalReplenished, BonusBalance: a.BonusBalance,
		}, a.BonusState)
	}
	for _, s := range f.Services {
		b.AddService(Service{
			ID: s.ID, Name: s.Name, OwnerID: s.OwnerID, PaymentTerm: PaymentTerm(s.PaymentTerm),
			AutoProlong: s.AutoProlong, CreatedAt: s.CreatedAt.UTC(), ExpiresAt: s.ExpiresAt.UTC(),
			Status: ServiceStatus(s.Status), Price: s.Price,
		})
	}
	for _, t := range f.Transactions {
		var performedAt *time.Time
		if t.PerformedAt != nil {
			p := t.PerformedAt.UTC()
			performedAt = &p
		}
		b.AddTransaction(Transaction{
			ID: t.ID, ServiceID: t.ServiceID, Amount: t.Amount, PerformedAt: performedAt,
			CreatedAt: t.CreatedAt.UTC(), Type: TxType(t.Type),
		}, t.Status)
	}
	return b.Dataset()
}
