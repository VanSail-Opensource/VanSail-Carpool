package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func pricingTestAdmin(t *testing.T, s *Store) model.User {
	t.Helper()
	u, _, err := s.UpsertTelegramUser(context.Background(), model.TelegramProfile{ID: 9910123, FirstName: "Pricing administrator"}, true)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestPricingExplicitActivationAndVersionFence(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	actor := pricingTestAdmin(t, s)
	a := saveTestCombo(t, s, "A", 1000, 30)
	b := saveTestCombo(t, s, "B", 2000, 30)
	p, err := s.PricingPolicy(ctx, "combo", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.DynamicEnabled || p.Version != 0 {
		t.Fatal("migration activated pricing")
	}
	p.DynamicEnabled = true
	p.PurchaseMode = "DYNAMIC"
	p.MaxIncreaseBPS = 1000
	saved, err := s.SavePricingPolicy(ctx, actor.ID, "activate A only", p, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SavePricingPolicy(ctx, actor.ID, "stale save", p, time.Now()); !errors.Is(err, pricing.VersionConflict) {
		t.Fatalf("stale version accepted: %v", err)
	}
	other, err := s.PricingPolicy(ctx, "combo", b.ID)
	if err != nil || other.DynamicEnabled {
		t.Fatal("unrelated product changed", err)
	}
	saved.DynamicEnabled = false
	saved.PurchaseMode = "FIXED"
	if _, err = s.SavePricingPolicy(ctx, actor.ID, "return future offers to fixed", saved, time.Now()); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE action='pricing.policy'`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("audit=%d %v", count, err)
	}
}
