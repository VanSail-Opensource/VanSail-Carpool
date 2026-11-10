package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// The bounded prefetch path is disabled here (sequential, zero lead window) so
// duplicate late workers deterministically exercise the current-remainder rule
// instead of legitimately prepaying the next hour near a UTC boundary.
func TestHourlyLateAndDuplicateWorkersChargeOnlyCurrentRemainder(t *testing.T) {
	withHourlyPrefetchLead(t, 0)
	s, policy, input := hourlyBillingFixture(t)
	before := hourlyBillingNow(t, s)
	e := makeDueHourlyBillingFixture(t, s, input, policy, before.Add(-4*time.Hour))
	guards := hourlyBillingDelivery(t, s, e.ID)
	start, done := make(chan struct{}), make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := s.SettleHourlyOwnerAvailable(context.Background(), input.OwnerID, input.AccountGeneration, policy, guards, before.Add(-2*time.Hour))
			done <- err
		}()
	}
	close(start)
	for range 2 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	var receipts int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM hourly_settlements WHERE enrollment_id=$1`, e.ID).Scan(&receipts); err != nil || receipts != 1 {
		t.Fatalf("late workers billed missed/duplicate hours: %d %v", receipts, err)
	}
	r, err := scanHourlySettlement(s.DB().QueryRow(`SELECT `+hourlySettlementColumns+` FROM hourly_settlements WHERE enrollment_id=$1`, e.ID))
	if err != nil || r.ChargedFrom.Before(before) || !r.HourStart.Equal(r.ChargedFrom.Truncate(time.Hour)) {
		t.Fatalf("late charge used stale scheduled time: %+v %v", r, err)
	}
	updated, err := s.HourlyEnrollment(context.Background(), e.ID)
	if err != nil || !updated.ActivatedAt.Equal(*e.ActivatedAt) || updated.PeriodID != e.PeriodID || !updated.PaidThrough.Equal(r.PaidUntil) {
		t.Fatalf("renewal replaced activation/accounting identity: %+v %v", updated, err)
	}
	if hourlyBillingBalance(t, s, input.OwnerID) != 100000-r.ChargedTXBMinor {
		t.Fatal("late settlement wallet differs from one receipt")
	}
}

func TestHourlyRenewalsSpendDeterministicallyWithoutOverdraft(t *testing.T) {
	t.Parallel()
	s, policy, first := hourlyBillingFixture(t)
	ctx := context.Background()
	if _, err := s.SaveHourlyOffer(ctx, first.OwnerID, model.HourlyOffer{GroupID: first.GroupID, Enabled: true, HourlyPriceTXBMinor: 1}, 1); err != nil {
		t.Fatal(err)
	}
	first.ExpectedOfferRevision = 2
	now := hourlyBillingNow(t, s)
	a := makeDueHourlyBillingFixture(t, s, first, policy, now.Add(-5*time.Hour))
	second := first
	second.GroupID, second.ActivationKey, second.ExpectedOfferRevision = "billing-hourly-b", "activation-two", 1
	addHourlyBillingOffer(t, s, first.OwnerID, second.GroupID, 100)
	b := makeDueHourlyBillingFixture(t, s, second, policy, now.Add(-4*time.Hour))
	if _, err := s.DB().Exec(`UPDATE balances SET txb_minor=1 WHERE user_id=$1`, first.OwnerID); err != nil {
		t.Fatal(err)
	}
	result, err := s.SettleHourlyOwnerAvailable(ctx, first.OwnerID, first.AccountGeneration, policy, hourlyBillingDelivery(t, s, a.ID, b.ID), time.Time{})
	if err != nil || len(result.Settlements) != 1 || result.Settlements[0].EnrollmentID != a.ID || hourlyBillingBalance(t, s, first.OwnerID) != 0 {
		t.Fatalf("deterministic first squad did not win: %+v %v", result, err)
	}
	suspended, err := s.HourlyEnrollment(ctx, b.ID)
	if err != nil || suspended.Status != model.HourlyFundingSuspended || suspended.CooldownUntil != nil || !suspended.PaidThrough.Equal(*b.PaidThrough) {
		t.Fatalf("funding suspension charged/moved cooldown: %+v %v", suspended, err)
	}
	if _, err := s.AdjustBalance(ctx, first.OwnerID, 100000, "fund-again", "fixture", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SettleHourlyOwnerAvailable(ctx, first.OwnerID, first.AccountGeneration, policy, hourlyBillingDelivery(t, s, a.ID, b.ID), time.Time{}); err != nil {
		t.Fatal(err)
	}
	suspended, _ = s.HourlyEnrollment(ctx, b.ID)
	if suspended.Status != model.HourlyFundingSuspended {
		t.Fatal("funding suspension automatically resumed without explicit eligibility")
	}
}
