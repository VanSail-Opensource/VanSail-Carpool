package renewal

import (
	"testing"
	"time"
)

func TestRenewalQuoteCeilingAndImmutableWindow(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	notice := now.Add(-24 * time.Hour)
	c := Contract{ID: "contract", Price: 1000, Ceiling: 1100, MaxChangeBPS: 1000, Version: 1, Mode: "DYNAMIC"}
	q := Quote{ContractID: c.ID, ContractVersion: 1, Price: 1050, State: "AUTHORIZED", AuthorizedAt: &notice, SettleAfter: now, ValidUntil: now.Add(time.Hour)}
	if err := q.Settleable(c, now); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*Quote)
	}{{"above_ceiling", func(q *Quote) { q.Price = 1101 }}, {"expired", func(q *Quote) { q.ValidUntil = now }}, {"superseded", func(q *Quote) { q.ContractVersion = 2 }}, {"pending", func(q *Quote) { q.State = "NOTICE_PENDING" }}} {
		t.Run(tc.name, func(t *testing.T) {
			copy := q
			tc.change(&copy)
			if copy.Settleable(c, now) == nil {
				t.Fatal("unauthorized renewal accepted")
			}
		})
	}
}

func TestLowRenewalProbabilityDoesNotGrantAnIndividualDiscount(t *testing.T) {
	c := Contract{Mode: "DYNAMIC", Price: 1000, Ceiling: 1100, MaxChangeBPS: 1000}
	best, candidates, err := Recommend(Inputs{Contract: c, PublicPrice: 1000, BaseProbability: .01, DiscountBPS: 5000, Validated: false})
	if err != nil || best.Price != 1000 {
		t.Fatal(best, err)
	}
	for _, candidate := range candidates {
		if candidate.Eligible && candidate.Price < 1000 {
			t.Fatal("propensity manufactured a discount", candidate)
		}
	}
}
