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
	err := s.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_schema WHERE name IN ('purchase_rollovers','rollover_member_daily_rollups')`).Scan(&count)
	if err != nil || count != 0 {
		t.Fatalf("retired tables: count=%d error=%v", count, err)
	}
	for _, table := range []string{"combos", "purchases"} {
		err = s.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name LIKE '%rollover%'`, table).Scan(&count)
		if err != nil || count != 0 {
			t.Fatalf("retired columns: table=%s count=%d error=%v", table, count, err)
		}
	}
}

func TestRetirementCancelsCalculatedCreditAndPreservesWallet(t *testing.T) {
	t.Parallel()
	s := newHistoricalRewardStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	c := saveTestCombo(t, s, "Retirement fixture", 100, 1)
	u, p := createAdminWorkflowPurchase(t, s, 48921, c, now)
	seedHistoricalRewardSettlement(t, s, p.ID, now, false)
	if _, err := s.DB().Exec(`UPDATE purchase_rollovers SET status='calculated',allocated_traffic_bytes=1000,used_traffic_bytes=100,eligible_unused_bytes=900 WHERE purchase_id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`CREATE TABLE rollover_member_daily_rollups(local_date TEXT,user_id TEXT,settlement_count INTEGER,credited_txb_minor INTEGER,allocated_traffic_bytes INTEGER,used_traffic_bytes INTEGER,updated_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	before := adminWorkflowBalance(t, s, u.ID)
	raw, err := migrations.ReadFile("migrations/106_retire_rollover.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB().ExecContext(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	if after := adminWorkflowBalance(t, s, u.ID); after != before {
		t.Fatalf("retirement changed wallet: %d -> %d", before, after)
	}
	var credits int
	if err = s.DB().QueryRow(`SELECT COUNT(*) FROM ledger_entries WHERE reference_id=? AND kind='rollover_credit'`, p.ID).Scan(&credits); err != nil || credits != 0 {
		t.Fatal("retirement created credit", credits, err)
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
