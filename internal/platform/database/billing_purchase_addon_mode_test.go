package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPurchaseAddonRejectsCurrentPAYGBillingMode(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	user := createTestUser(t, store, 69_031)
	combo := saveTestCombo(t, store, "addon-mode", 100, 30)
	addon := saveTestSquad(t, store, "addon-mode-change", 10, true)
	if _, err := store.AdjustBalance(ctx, user.ID, 1000, "mode-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "source"}, now)
	if err != nil {
		t.Fatal(err)
	}
	input := PurchaseAddonInput{UserID: user.ID, PurchaseID: purchase.ID, AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "addition"}
	if _, err := store.QuotePurchaseAddons(ctx, input, now); err != nil {
		t.Fatal(err)
	}
	// Merchandising may change after the service reads its live catalog. The
	// settlement transaction must not grant metered access for a fixed fee.
	if _, err := store.DB().ExecContext(ctx, `UPDATE squad_product_overrides SET billing_mode='payg',payg_price_minor=100 WHERE group_id=$1`, addon.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.QuotePurchaseAddons(ctx, input, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("quote after PAYG change = %v, want ErrNotFound", err)
	}
	if _, err := store.AddPurchaseAddons(ctx, input, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("settlement after PAYG change = %v, want ErrNotFound", err)
	}
	balance, err := store.Balance(ctx, user.ID)
	if err != nil || balance.Minor != "900" {
		t.Fatalf("rejected addition changed wallet: %s, %v", balance.Minor, err)
	}
	var adjustments int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM purchase_addon_adjustments WHERE purchase_id=$1`, purchase.ID).Scan(&adjustments); err != nil || adjustments != 0 {
		t.Fatalf("rejected addition recorded %d adjustments, %v", adjustments, err)
	}
}
