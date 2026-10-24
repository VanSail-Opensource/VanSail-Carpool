package database

import (
	"context"
	"testing"
	"time"
)

func TestAdminEntitlementFiltersUseDisplayValidityWithoutSettling(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	combo := saveTestCombo(t, store, "validity", 100, 30)
	cases := []struct {
		name, raw, display string
		from, until        time.Time
	}{
		{"boundary-active", "active", "expired", now.Add(-time.Hour), now},
		{"boundary-activating", "activating", "expired", now.Add(-time.Hour), now},
		{"overdue-queued", "queued", "expired", now.Add(-2 * time.Hour), now.Add(-time.Hour)},
		{"future-active", "active", "queued", now.Add(time.Hour), now.Add(2 * time.Hour)},
		{"future-activating", "activating", "queued", now.Add(time.Hour), now.Add(2 * time.Hour)},
		{"current", "active", "active", now, now.Add(time.Hour)},
		{"applying", "activating", "activating", now, now.Add(time.Hour)},
		{"cancelled", "cancelled", "cancelled", now.Add(-time.Hour), now},
		{"failed", "failed", "failed", now.Add(-time.Hour), now},
		{"expired", "expired", "expired", now, now.Add(time.Hour)},
	}
	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user, purchase := createAdminWorkflowPurchase(t, store, 82_000+int64(index), combo, now)
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status=?,valid_from=?,valid_until=? WHERE id=?`,
				tc.raw, stamp(tc.from), stamp(tc.until), purchase.ID); err != nil {
				t.Fatal(err)
			}
			seedPendingRewardRollover(t, store, purchase.ID, now)
			before := adminWorkflowBalance(t, store, user.ID)
			for _, filter := range []string{"active", "activating", "queued", "expired", "cancelled", "failed"} {
				items, _, err := store.listAdminPurchasesPageAt(ctx, "", purchase.ID, filter, 25, now)
				if err != nil {
					t.Fatal(err)
				}
				if (len(items) == 1) != (filter == tc.display) {
					t.Fatalf("filter %s returned %d records, want display %s", filter, len(items), tc.display)
				}
				if len(items) == 1 && items[0].Status != tc.raw {
					t.Fatalf("raw status changed: %+v", items[0])
				}
			}
			rollover, err := store.RolloverByPurchase(ctx, purchase.ID)
			if err != nil || rollover.Status != "pending" || before != adminWorkflowBalance(t, store, user.ID) {
				t.Fatalf("display read modified settlement: %+v, %v", rollover, err)
			}
		})
	}
}

func TestAdminValidityReadsPreservePendingPredecessorAndPaidSuccessor(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	combo := saveTestCombo(t, store, "successor-validity", 100, 30)
	user, predecessor := createAdminWorkflowPurchase(t, store, 82_100, combo, now.AddDate(0, 0, -31))
	seedPendingRewardRollover(t, store, predecessor.ID, now)
	successor, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "paid-successor"}, now)
	if err != nil {
		t.Fatal(err)
	}
	successor = activateAdminWorkflowPurchase(t, store, successor.ID, now)
	items, _, err := store.listAdminPurchasesPageAt(ctx, "", user.ID, "active", 25, now)
	if err != nil || len(items) != 1 || items[0].ID != successor.ID {
		t.Fatalf("current inventory = %+v, %v", items, err)
	}
	current, _, err := store.ActiveAndQueuedPurchases(ctx, user.ID, now)
	if err != nil || current == nil || current.ID != successor.ID {
		t.Fatalf("current summary = %+v, %v", current, err)
	}
	old, err := store.PurchaseByID(ctx, predecessor.ID)
	if err != nil || old.Status != "active" {
		t.Fatalf("pending predecessor was rewritten: %+v, %v", old, err)
	}
	rollover, err := store.RolloverByPurchase(ctx, predecessor.ID)
	if err != nil || rollover.Status != "pending" {
		t.Fatalf("pending settlement was changed: %+v, %v", rollover, err)
	}
}
