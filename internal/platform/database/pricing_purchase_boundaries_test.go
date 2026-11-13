package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPricingPurchasePreparationUsesExactCurrentExpiry(t *testing.T) {
	for _, precision := range []time.Duration{0, 100 * time.Millisecond} {
		for _, live := range []bool{false, true} {
			s, user, _, source, policy, now := pricingRenewalFixture(t)
			ctx := context.Background()
			at := now.Truncate(time.Second).Add(precision)
			until := at
			if live {
				until = until.Add(time.Nanosecond)
			}
			if _, err := s.db.ExecContext(ctx, `UPDATE purchases SET valid_from=$1,valid_until=$2 WHERE id=$3`, stamp(at.Add(-time.Hour)), stamp(until), source.ID); err != nil {
				t.Fatal(err)
			}
			tx, err := s.db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			decision, decisionErr := s.queuedRenewalDecisionTx(ctx, tx, user.ID, at)
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			if live && !errors.Is(decisionErr, ErrPricingDecisionPending) || !live && (decisionErr != nil || decision != nil) {
				t.Fatal("pricing selected the wrong current term", precision, live, decision, decisionErr)
			}
			if err := s.PreparePurchasePricing(ctx, PurchaseInput{UserID: user.ID, ComboID: policy.ProductID}, at); err != nil {
				t.Fatal(err)
			}
			prepared, err := s.preparePersonalRenewalDecision(ctx, user.ID, at)
			if err != nil || (prepared != nil) != live {
				t.Fatal("member renewal projection selected an expired source", precision, live, prepared, err)
			}
			var contracts int
			if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_contracts WHERE purchase_id=$1`, source.ID).Scan(&contracts); err != nil || (contracts > 0) != live {
				t.Fatal("expired preparation created a renewal obligation", precision, live, contracts, err)
			}
		}
	}
}

func TestPricingRenewalPreparationWindowUsesExactPurchaseStamps(t *testing.T) {
	s, _, _, source, _, now := pricingRenewalFixture(t)
	ctx := context.Background()
	at := now.Truncate(time.Second)
	from, through := at.Add(-72*time.Hour), at.Add(7*24*time.Hour)
	for _, test := range []struct {
		name  string
		until time.Time
		want  bool
	}{
		{"expired retry window", from, false},
		{"first retry nanosecond", from.Add(time.Nanosecond), true},
		{"exact preparation boundary", through, true},
		{"after preparation boundary", through.Add(time.Nanosecond), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := s.db.ExecContext(ctx, `UPDATE purchases SET valid_from=$1,valid_until=$2 WHERE id=$3`, stamp(from.Add(-time.Hour)), stamp(test.until), source.ID); err != nil {
				t.Fatal(err)
			}
			ids, err := s.DuePricingRenewals(ctx, at)
			if err != nil || (len(ids) == 1 && ids[0] == source.ID) != test.want || len(ids) > 1 {
				t.Fatal("pricing preparation window changed by stamp precision", ids, test.want, err)
			}
		})
	}
}
