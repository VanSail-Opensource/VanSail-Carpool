package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCurrentSchemaHasNoOperationalRollover(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	var count int
	err := s.DB().QueryRow(`SELECT COUNT(*) FROM pg_class WHERE relnamespace='public'::regnamespace AND relname IN ('purchase_rollovers','rollover_member_daily_rollups')`).Scan(&count)
	if err != nil || count != 0 {
		t.Fatalf("retired tables: count=%d error=%v", count, err)
	}
	for _, table := range []string{"combos", "purchases"} {
		err = s.DB().QueryRow(`SELECT COUNT(*) FROM pg_attribute WHERE attrelid=to_regclass($1) AND attnum>0 AND NOT attisdropped AND attname ILIKE '%rollover%'`, table).Scan(&count)
		if err != nil || count != 0 {
			t.Fatalf("retired columns: table=%s count=%d error=%v", table, count, err)
		}
	}
}

func TestRenewalEnablementDoesNotRequireFundsAndSettlementDoes(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	u := createTestUser(t, s, 48922)
	c := saveTestCombo(t, s, "Unfunded renewal", 100, 1)
	if _, err := s.AdjustBalance(ctx, u.ID, 100, "renewal-fixture", "fixture", now); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePurchase(ctx, PurchaseInput{UserID: u.ID, ComboID: c.ID, IdempotencyKey: "renewal-fixture"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetAutoRenewal(ctx, u.ID, p.ID, true, now); err != nil {
		t.Fatal("unfunded enablement", err)
	}
	plan, err := s.AutoRenewalPlan(ctx, u.ID, p.ID, now)
	if err != nil || plan.IneligibleReason != "" || plan.NetMinor != 100 {
		t.Fatal(plan, err)
	}
	if _, err = s.CommitAutoRenewal(ctx, p.ID, p.ValidUntil); !errors.Is(err, ErrInsufficientBalance) {
		t.Fatal("unfunded settlement", err)
	}
	if b, err := s.Balance(ctx, u.ID); err != nil || b.MinorInt64() != 0 {
		t.Fatal(b, err)
	}
}
