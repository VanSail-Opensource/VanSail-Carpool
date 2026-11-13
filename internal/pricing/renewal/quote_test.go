package renewal

import (
	"testing"
	"time"
)

func TestRenewalQuoteCeilingAndImmutableWindow(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	notice := now.Add(-24 * time.Hour)
	c := Contract{ID: "contract", PurchaseID: "purchase", Price: 1000, Ceiling: 1100, MaxChangeBPS: 1000, Version: 1, Mode: "DYNAMIC", Addons: []AddonTerm{{ID: "squad", Price: 200}}}
	q := Quote{ContractID: c.ID, PurchaseID: c.PurchaseID, ContractVersion: 1, Price: 1050, State: "AUTHORIZED", AuthorizedAt: &notice, SettleAfter: now, ValidUntil: now.Add(time.Hour)}
	if err := q.Settleable(c, now); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*Quote)
	}{
		{"above_ceiling", func(q *Quote) { q.Price = 1101 }},
		{"negative_price", func(q *Quote) { q.Price = -1 }},
		{"below_paid_addon_basis", func(q *Quote) { q.Price = 199 }},
		{"different_purchase", func(q *Quote) { q.PurchaseID = "another-purchase" }},
		{"expired", func(q *Quote) { q.ValidUntil = now }},
		{"superseded", func(q *Quote) { q.ContractVersion = 2 }},
		{"pending", func(q *Quote) { q.State = "NOTICE_PENDING" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := q
			tc.change(&copy)
			if copy.Settleable(c, now) == nil {
				t.Fatal("unauthorized renewal accepted")
			}
		})
	}
	c.MaxChangeBPS = -1
	if q.Settleable(c, now) == nil {
		t.Fatal("malformed stored contract accepted")
	}
}
