package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestHourlyPausingNewSalesDoesNotCancelExistingRenewals(t *testing.T) {
	t.Parallel()
	s, policy, input := hourlyBillingFixture(t)
	ctx := context.Background()
	e := makeDueHourlyBillingFixture(t, s, input, policy, hourlyBillingNow(t, s).Add(-4*time.Hour))
	if _, err := s.SaveHourlyOffer(ctx, input.OwnerID, model.HourlyOffer{GroupID: input.GroupID, Enabled: false, HourlyPriceTXBMinor: 100}, 1); err != nil {
		t.Fatal(err)
	}
	record, err := s.CompareAndSwapHourlyPolicy(ctx, input.OwnerID, policy.Version, "sales-paused")
	if err != nil {
		t.Fatal(err)
	}
	policy.Version, policy.Policy.SalesEnabled = record.Version, false
	result, err := s.SettleHourlyOwnerAvailable(ctx, input.OwnerID, input.AccountGeneration, policy, hourlyBillingDelivery(t, s, e.ID), time.Now())
	if err != nil || len(result.Settlements) != 1 || result.Enrollments[0].Status != model.HourlyActive {
		t.Fatalf("paused new sales cancelled existing term: %+v %v", result, err)
	}
	input.ActivationKey, input.ExpectedPolicyVersion, input.ExpectedOfferRevision = "new-paused-sale", record.Version, 2
	if _, err := s.PrepareHourlyActivation(ctx, input, policy, time.Now()); !errors.Is(err, ErrHourlySalesDisabled) {
		t.Fatalf("paused policy admitted a new sale: %v", err)
	}
}

func TestAutomaticLegacyRenewalPopulatesPurchasedDuration(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ctx := context.Background()
	u := createTestUser(t, s, 89713)
	combo := saveTestCombo(t, s, "legacy-auto-duration", 100, 30)
	if _, err := s.AdjustBalance(ctx, u.ID, 1000, "auto-duration-seed", "fixture", time.Now()); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePurchase(ctx, PurchaseInput{UserID: u.ID, ComboID: combo.ID, IdempotencyKey: "auto-duration"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetAutoRenewal(ctx, u.ID, p.ID, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	next, err := s.CommitAutoRenewal(ctx, p.ID, p.ValidUntil)
	if err != nil || next.PurchasedTermSeconds == nil || *next.PurchasedTermSeconds != 30*24*60*60 {
		t.Fatalf("automatic renewal lost accepted duration: %+v %v", next, err)
	}
}
