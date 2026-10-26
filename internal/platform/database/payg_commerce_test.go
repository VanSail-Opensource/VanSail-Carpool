package database

import (
	"context"
	"slices"
	"testing"
	"time"
)

func TestPAYGConversionPreservesPaidTermsAndFiltersRenewal(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := createTestUser(t, s, 80491)
	combo := saveTestCombo(t, s, "PAYG conversion", 100, 1, "kept", "converted")
	if _, err := s.AdjustBalance(ctx, user.ID, 1000, "seed", "seed", time.Now()); err != nil {
		t.Fatal(err)
	}
	paid, err := s.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "active"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	queued, err := s.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "queued"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveSquadProduct(ctx, SquadProductInput{GroupID: "converted", UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 100}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{paid.ID, queued.ID} {
		p, err := s.PurchaseByID(ctx, id)
		if err != nil || !slices.Contains(p.SquadUUIDs, "converted") {
			t.Fatal("paid squad revoked", p, err)
		}
	}
	renewed, err := s.Renew(ctx, RenewalInput{UserID: user.ID, PurchaseID: paid.ID, TermCount: 1, IdempotencyKey: "renew"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(renewed.Purchases) != 1 || !slices.Equal(renewed.Purchases[0].SquadUUIDs, []string{"kept"}) {
		t.Fatal("renewal retained PAYG squad", renewed)
	}
}

func TestPAYGLastSquadExpiresWithoutCredit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := createTestUser(t, s, 80492)
	combo := saveTestCombo(t, s, "Empty PAYG renewal", 100, 1, "converted")
	now := time.Now().UTC()
	if _, err := s.AdjustBalance(ctx, user.ID, 100, "seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "active"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SaveSquadProduct(ctx, SquadProductInput{GroupID: "converted", UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 100}); err != nil {
		t.Fatal(err)
	}
	if err = s.ExpireDuePurchase(ctx, p.ID, p.ValidUntil); err != nil {
		t.Fatal(err)
	}
	after, err := s.PurchaseByID(ctx, p.ID)
	if err != nil || after.Status != "expired" || after.AutoRenewEnabled {
		t.Fatal(after, err)
	}
	if balance, err := s.Balance(ctx, user.ID); err != nil || balance.MinorInt64() != 0 {
		t.Fatal("expiry credited wallet", balance, err)
	}
}
