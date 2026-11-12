package database

import (
	"context"
	"testing"
	"time"
)

func TestPurchaseAddonProrationRetainsPurchasedDurationAfterCatalogEdit(t *testing.T) {
	t.Parallel()
	for _, validity := range []int{1, 60} {
		store := newTestStore(t)
		ctx := context.Background()
		user := createTestUser(t, store, 69_020)
		now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		combo := saveTestCombo(t, store, "duration-snapshot", 1_000, 30)
		addon := saveTestSquad(t, store, "duration-addon", 301, true)
		if _, err := store.AdjustBalance(ctx, user.ID, 5_000, "duration-seed", "seed", now); err != nil {
			t.Fatal(err)
		}
		purchase, err := store.CreatePurchase(ctx, PurchaseInput{
			UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "duration-source",
		}, now)
		if err != nil || purchase.PurchasedTermSeconds == nil {
			t.Fatal("purchase did not retain original duration", err)
		}
		if _, err := store.DB().ExecContext(ctx, `UPDATE combos SET validity_days=$1 WHERE id=$2`, validity, combo.ID); err != nil {
			t.Fatal(err)
		}
		input := PurchaseAddonInput{UserID: user.ID, PurchaseID: purchase.ID,
			AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "duration-addition"}
		quotedAt := now.Add(15 * 24 * time.Hour)
		quote, err := store.QuotePurchaseAddons(ctx, input, quotedAt)
		if err != nil || quote.PriceTXBMinor != 151 {
			t.Fatalf("catalog validity %d changed paid-term proration: %d, %v", validity, quote.PriceTXBMinor, err)
		}
		updated, err := store.AddPurchaseAddons(ctx, input, quotedAt)
		if err != nil || updated.PriceTXBMinor != 1_151 {
			t.Fatalf("catalog validity %d changed debit: %d, %v", validity, updated.PriceTXBMinor, err)
		}
		balance, err := store.Balance(ctx, user.ID)
		if err != nil || balance.Minor != "3849" {
			t.Fatal("incorrect exact balance after addon", balance.Minor, err)
		}
	}
}
