package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNativePurchaseCompletionRequiresCurrentMaterialAndStatus(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 11, 13, 17, 0, 0, 0, time.UTC)
	user := createTestUser(t, store, 89113)
	combo := saveTestCombo(t, store, "Native completion", 100, 30)
	if _, err := store.AdjustBalance(ctx, user.ID, 1000, "native-completion-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	original, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "native-completion"}, now)
	if err != nil {
		t.Fatal(err)
	}
	// Keep UpdatedAt unchanged deliberately: material comparison also protects
	// same-clock edits and changes to effective live catalog values.
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=$1 WHERE id=$2`, stamp(original.ValidUntil.Add(time.Hour)), original.ID); err != nil {
		t.Fatal(err)
	}
	for _, success := range []bool{true, false} {
		if err := store.MarkPurchaseSyncResultIfUnchanged(ctx, original, user.AccountGeneration, success, now); !errors.Is(err, ErrConflict) {
			t.Fatalf("stale material completion (%t) = %v", success, err)
		}
	}
	current, err := store.PurchaseByID(ctx, original.ID)
	if err != nil || current.Status != "activating" {
		t.Fatalf("rejected completion changed target: %+v, %v", current, err)
	}
	wrongOwner := current
	wrongOwner.UserID = createTestUser(t, store, 89114).ID
	if err := store.MarkPurchaseSyncResultIfUnchanged(ctx, wrongOwner, user.AccountGeneration, true, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("foreign owner completion = %v", err)
	}
	if err := store.MarkPurchaseSyncResultIfUnchanged(ctx, current, user.AccountGeneration, true, now); err != nil {
		t.Fatal(err)
	}
	// Completion used the same clock as creation. Status comparison must still
	// reject the older activating snapshot, including a late failure response.
	for _, success := range []bool{true, false} {
		if err := store.MarkPurchaseSyncResultIfUnchanged(ctx, current, user.AccountGeneration, success, now); !errors.Is(err, ErrConflict) {
			t.Fatalf("stale status completion (%t) = %v", success, err)
		}
	}
	active, err := store.PurchaseByID(ctx, current.ID)
	if err != nil || active.Status != "active" {
		t.Fatalf("accepted completion = %+v, %v", active, err)
	}
	for _, success := range []bool{true, false} {
		if err := store.MarkPurchaseSyncResultIfUnchanged(ctx, active, user.AccountGeneration, success, now.Add(time.Minute)); err != nil {
			t.Fatalf("current active replay (%t) = %v", success, err)
		}
	}
	retained, err := store.PurchaseByID(ctx, current.ID)
	if err != nil || !retained.UpdatedAt.Equal(active.UpdatedAt) || retained.Status != "active" {
		t.Fatalf("active replay changed receipt: %+v, %v", retained, err)
	}
	var allowances, events int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ip_lookup_allowances WHERE purchase_id=$1`, active.ID).Scan(&allowances); err != nil || allowances != 1 {
		t.Fatalf("activation allowances = %d, %v", allowances, err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notification_events WHERE event_key=$1`, "activation:"+active.ID).Scan(&events); err != nil || events != 1 {
		t.Fatalf("activation events = %d, %v", events, err)
	}
}

func TestNativePurchaseCompletionRejectsUnavailableOwnerAndTerm(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 11, 13, 18, 0, 0, 0, time.UTC)
	user := createTestUser(t, store, 89115)
	combo := saveTestCombo(t, store, "Native completion eligibility", 100, 30)
	if _, err := store.AdjustBalance(ctx, user.ID, 1000, "native-eligibility-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "native-eligibility"}, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Time{purchase.ValidFrom.Add(-time.Nanosecond), purchase.ValidUntil} {
		if err := store.MarkPurchaseSyncResultIfUnchanged(ctx, purchase, user.AccountGeneration, true, at); !errors.Is(err, ErrConflict) {
			t.Fatalf("ineligible term completion = %v", err)
		}
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET account_status='purging' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkPurchaseSyncResultIfUnchanged(ctx, purchase, user.AccountGeneration, true, now); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatalf("purging owner completion = %v", err)
	}
	current, err := store.PurchaseByID(ctx, purchase.ID)
	if err != nil || current.Status != "activating" || !current.UpdatedAt.Equal(purchase.UpdatedAt) {
		t.Fatalf("rejected completion changed target: %+v, %v", current, err)
	}
	var allowances int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ip_lookup_allowances WHERE purchase_id=$1`, purchase.ID).Scan(&allowances); err != nil || allowances != 0 {
		t.Fatalf("rejected activation allowance = %d, %v", allowances, err)
	}
}

func TestNativePurchaseCompletionRequiresExactAccountGeneration(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 11, 13, 19, 0, 0, 0, time.UTC)
	user := createTestUser(t, store, 89116)
	combo := saveTestCombo(t, store, "Native generation", 100, 30)
	if _, err := store.AdjustBalance(ctx, user.ID, 1000, "native-generation-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "native-generation"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE users SET account_generation=account_generation+1 WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	for _, success := range []bool{true, false} {
		if err = store.MarkPurchaseSyncResultIfUnchanged(ctx, purchase, user.AccountGeneration, success, now); !errors.Is(err, ErrAccountUnavailable) {
			t.Fatalf("prior generation completion (%t) = %v", success, err)
		}
	}
	current, err := store.PurchaseByID(ctx, purchase.ID)
	if err != nil || current.Status != "activating" || !current.UpdatedAt.Equal(purchase.UpdatedAt) {
		t.Fatal("prior generation changed target", current, err)
	}
	var allowances int
	if err = store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ip_lookup_allowances WHERE purchase_id=$1`, purchase.ID).Scan(&allowances); err != nil || allowances != 0 {
		t.Fatal("prior generation issued allowance", allowances, err)
	}
	if err = store.MarkPurchaseSyncResultIfUnchanged(ctx, current, user.AccountGeneration+1, true, now); err != nil {
		t.Fatal("current generation completion failed", err)
	}
	active, err := store.PurchaseByID(ctx, purchase.ID)
	if err != nil || active.Status != "active" {
		t.Fatal("current generation did not activate", active, err)
	}
	if err = store.MarkPurchaseSyncResultIfUnchanged(ctx, active, user.AccountGeneration, false, now); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("stale generation received active completion replay", err)
	}
}
