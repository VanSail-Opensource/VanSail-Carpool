package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestProratedAddonPriceRoundsAndCapsExtendedTerms(t *testing.T) {
	t.Parallel()
	original := 30 * 24 * time.Hour
	cases := []struct {
		name      string
		remaining time.Duration
		want      int64
	}{
		{name: "rounds up", remaining: 15 * 24 * time.Hour, want: 151},
		{name: "caps extension", remaining: 45 * 24 * time.Hour, want: 301},
		{name: "expires", remaining: 0, want: 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := proratedAddonPrice(301, test.remaining, original)
			if err != nil || got != test.want {
				t.Fatalf("proratedAddonPrice() = (%d, %v), want %d", got, err, test.want)
			}
		})
	}
}

func TestAddPurchaseAddonsDebitsOnceAndFeedsFutureTotals(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 69_001)
	addon := saveTestSquad(t, store, "added-squad", 301, true)
	combo := saveTestCombo(t, store, "addition-combo", 1_000, 30)
	now := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 5_000, "addition-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "addition-source"}, now)
	if err != nil {
		t.Fatal(err)
	}
	quotedAt := now.Add(15 * 24 * time.Hour)
	quote, err := store.QuotePurchaseAddons(ctx, PurchaseAddonInput{UserID: user.ID, PurchaseID: purchase.ID, AddonSquadIDs: []string{addon.ID}}, quotedAt)
	if err != nil || quote.PriceTXBMinor != 151 {
		t.Fatalf("QuotePurchaseAddons() = (%+v, %v), want 151", quote, err)
	}
	updated, err := store.AddPurchaseAddons(ctx, PurchaseAddonInput{UserID: user.ID, PurchaseID: purchase.ID, AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "addition-commit"}, quotedAt)
	if err != nil || updated.PriceTXBMinor != 1_151 || !equalStrings(updated.SquadUUIDs, []string{addon.ID}) {
		t.Fatalf("AddPurchaseAddons() = (%+v, %v)", updated, err)
	}
	replayed, err := store.AddPurchaseAddons(ctx, PurchaseAddonInput{UserID: user.ID, PurchaseID: purchase.ID, AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "addition-commit"}, quotedAt)
	if err != nil || replayed.ID != updated.ID || replayed.PriceTXBMinor != updated.PriceTXBMinor {
		t.Fatalf("AddPurchaseAddons(replay) = (%+v, %v)", replayed, err)
	}
	var pending, notices int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*),SUM(CASE WHEN queued_at IS NULL THEN 1 ELSE 0 END)
		FROM user_notification_events WHERE source_kind='addon-activated'`).Scan(&notices, &pending); err != nil || notices != 1 || pending != 1 {
		t.Fatalf("add-on notice before sync = %d total, %d pending, %v", notices, pending, err)
	}
	if err := store.ReleaseUserSyncNotifications(ctx, user.ID, quotedAt); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notification_events
		WHERE source_kind='addon-activated' AND queued_at IS NOT NULL`).Scan(&notices); err != nil || notices != 1 {
		t.Fatalf("add-on notice after sync = %d, %v", notices, err)
	}
	plan, err := store.AutoRenewalPlan(ctx, user.ID, purchase.ID, quotedAt)
	if err != nil || plan.GrossMinor != 1_301 {
		t.Fatalf("AutoRenewalPlan() = (%+v, %v), want full-term 1301", plan, err)
	}
	if err := store.EnqueueDueEntitlementTransitions(ctx, purchase.ValidUntil); err != nil {
		t.Fatal(err)
	}
	var adjustments, debits, syncJobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM purchase_addon_adjustments`).Scan(&adjustments); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE kind='purchase_addon_debit'`).Scan(&debits); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_sync_user' AND payload=?`, `{"userId":"`+user.ID+`"}`).Scan(&syncJobs); err != nil {
		t.Fatal(err)
	}
	if adjustments != 1 || debits != 1 || syncJobs != 1 {
		t.Fatalf("addition effects adjustments=%d debits=%d syncJobs=%d", adjustments, debits, syncJobs)
	}
}

func TestPurchaseAddonsAllowQueuedTermsAndOwnerHeldFullSquads(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	owner := createTestUser(t, store, 69_002)
	limit := 1
	addon, err := store.SaveSquadProduct(ctx, SquadProductInput{GroupID: "full-owner-squad", Name: "full-owner-squad", Description: "test", PriceTXBMinor: 200, Visible: true, UpstreamPresent: true, StockLimit: &limit})
	if err != nil {
		t.Fatal(err)
	}
	combo := saveTestCombo(t, store, "full-owner-combo", 500, 30)
	if _, err := store.AdjustBalance(ctx, owner.ID, 2_000, "owner-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	active, err := store.CreatePurchase(ctx, PurchaseInput{UserID: owner.ID, ComboID: combo.ID, AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "owner-active"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreatePurchase(ctx, PurchaseInput{UserID: owner.ID, ComboID: combo.ID, AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "owner-queued"}, now.Add(time.Hour)); err != nil {
		t.Fatalf("CreatePurchase(owner queued full squad): %v", err)
	}
	if _, err := store.QuotePurchaseAddons(ctx, PurchaseAddonInput{UserID: owner.ID, PurchaseID: active.ID, AddonSquadIDs: []string{addon.ID}}, now.Add(time.Hour)); !errors.Is(err, ErrSquadAlreadyAdded) {
		t.Fatalf("QuotePurchaseAddons(duplicate) = %v, want ErrSquadAlreadyAdded", err)
	}
	liveAddon, err := store.SaveSquadProduct(ctx, SquadProductInput{GroupID: "live-extra", Name: "live-extra", PriceTXBMinor: 200, Visible: true, UpstreamPresent: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.QuotePurchaseAddons(ctx, PurchaseAddonInput{UserID: owner.ID, PurchaseID: active.ID, AddonSquadIDs: []string{liveAddon.ID}}, now.Add(time.Hour)); err != nil {
		t.Fatalf("QuotePurchaseAddons(queued) = %v, live addition should remain available", err)
	}
	other := createTestUser(t, store, 69_003)
	if _, err := store.AdjustBalance(ctx, other.ID, 2_000, "other-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreatePurchase(ctx, PurchaseInput{UserID: other.ID, ComboID: combo.ID, AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "other-full"}, now); !errors.Is(err, ErrStockUnavailable) {
		t.Fatalf("CreatePurchase(other full squad) = %v, want ErrStockUnavailable", err)
	}
}
