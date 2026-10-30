package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func saveDeletionGroup(t *testing.T, s *Store, id string) {
	t.Helper()
	if err := s.SaveXNodeGroup(context.Background(), xnode.Group{ID: id, Name: id, Enabled: true}); err != nil {
		t.Fatal(err)
	}
}

func TestGroupDeletionRemovesLiveCatalogAndPreservesPaidHistory(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	for _, id := range []string{"retired", "retained"} {
		saveDeletionGroup(t, s, id)
		saveTestSquad(t, s, id, 10, true)
	}
	c := saveTestCombo(t, s, "mixed", 100, 30, "retired", "retained")
	u := createTestUser(t, s, 49031)
	if _, err := s.AdjustBalance(ctx, u.ID, 1000, "seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePurchase(ctx, PurchaseInput{UserID: u.ID, ComboID: c.ID, IdempotencyKey: "mixed"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB().ExecContext(ctx, `UPDATE purchases SET status='queued' WHERE id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	usage, err := s.XNodeGroupUsage(ctx, now)
	if err != nil || usage["retired"].Members != 1 {
		t.Fatal(usage, err)
	}
	if err = s.DeleteXNodeGroup(ctx, "retired"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("queued access deleted", err)
	}
	if _, err = s.DB().ExecContext(ctx, `UPDATE purchases SET status='expired',entitlement_squad_uuids=NULL,combo_squad_uuids=NULL WHERE id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"retired", "retained"} {
		if _, err = s.DB().ExecContext(ctx, `INSERT INTO pricing_product_settings(product_type,product_id,fixed_price_minor,updated_by,updated_at) VALUES('addon',?,10,'fixture',?)`, id, stamp(now)); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.DeleteXNodeGroup(ctx, "retired"); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteXNodeGroup(ctx, "retired"); err != nil {
		t.Fatal("delete replay", err)
	}
	var live, snapshot string
	if err = s.DB().QueryRowContext(ctx, `SELECT included_squad_uuids FROM combos WHERE id=?`, c.ID).Scan(&live); err != nil || live != `["retained"]` {
		t.Fatal(live, err)
	}
	if err = s.DB().QueryRowContext(ctx, `SELECT combo_squad_uuids FROM purchases WHERE id=?`, p.ID).Scan(&snapshot); err != nil || snapshot != `["retired","retained"]` {
		t.Fatal("legacy entitlement changed", snapshot, err)
	}
	var prices, ledger int
	if err = s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_product_settings WHERE product_id='retained'`).Scan(&prices); err != nil || prices != 1 {
		t.Fatal(prices, err)
	}
	if err = s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_product_settings WHERE product_id='retired'`).Scan(&prices); err != nil || prices != 0 {
		t.Fatal(prices, err)
	}
	if err = s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE reference_id=? AND kind='purchase_debit'`, p.ID).Scan(&ledger); err != nil || ledger != 1 {
		t.Fatal("paid evidence lost", ledger, err)
	}
}

func TestGroupDeletionRejectsStaleServiceAndCatalogRead(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	saveDeletionGroup(t, s, "empty")
	version, _, err := s.XNodeCompileVersion(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	saveDeletionGroup(t, s, "other")
	if err = s.DeleteXNodeGroupAtVersion(ctx, "empty", version, nil); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("stale terms authorized deletion", err)
	}
}

func TestGroupDeletionCountsPAYGFundingCreditAndAccounting(t *testing.T) {
	ctx, now := context.Background(), time.Now().UTC()
	s, parent, _, a := paygBillingFixture(t)
	saveDeletionGroup(t, s, a.GroupID)
	if _, err := s.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete' WHERE id=?`, parent.UserID); err != nil {
		t.Fatal(err)
	}
	assertUsage := func(members int, pending bool) {
		t.Helper()
		usage, err := s.XNodeGroupUsage(ctx, now)
		if err != nil || usage[a.GroupID].Members != members || usage[a.GroupID].PendingAccounting != pending {
			t.Fatal(usage, err)
		}
	}
	assertUsage(1, false)
	if err := s.DeleteXNodeGroup(ctx, a.GroupID); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("funded access deleted", err)
	}
	if _, err := s.AdjustBalance(ctx, parent.UserID, -1000, "empty-wallet", "empty", now); err != nil {
		t.Fatal(err)
	}
	assertUsage(0, false)
	if _, err := s.DB().ExecContext(ctx, `UPDATE xnode_payg_accounts SET credit_bytes=1,credit_started_at=? WHERE proxy_user_id=?`, stamp(now), a.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	assertUsage(1, false)
	if _, err := s.DB().ExecContext(ctx, `UPDATE xnode_payg_accounts SET credit_started_at=? WHERE proxy_user_id=?`, stamp(now.Add(-72*time.Hour)), a.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	assertUsage(0, false)
	if _, err := s.DB().ExecContext(ctx, `UPDATE xnode_periods SET status='closing' WHERE id=?`, a.PeriodID); err != nil {
		t.Fatal(err)
	}
	assertUsage(0, true)
	if err := s.DeleteXNodeGroup(ctx, a.GroupID); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("unfinished accounting deleted", err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE xnode_periods SET status='closed' WHERE id=?`, a.PeriodID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteXNodeGroup(ctx, a.GroupID); err != nil {
		t.Fatal("dormant history blocks deletion", err)
	}
	if _, err := s.XNodePAYGAccount(ctx, a.ProxyUserID); err != nil {
		t.Fatal("historical tuple lost", err)
	}
}
