package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSquadStockRetainsExistingReservationsAfterLimitReduction(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		status string
		limit  int
		held   bool
	}{
		{name: "active overbooked", status: "active", limit: 1, held: true},
		{name: "activating sales closed", status: "activating", limit: 0, held: true},
		{name: "queued reservation", status: "queued", limit: 0, held: true},
		{name: "expired releases seat", status: "expired", limit: 1},
		{name: "cancelled releases seat", status: "cancelled", limit: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			store := newTestStore(t)
			now := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
			addon := saveTestSquad(t, store, "retained-stock", 100, true)
			combo := saveTestCombo(t, store, "retained-stock", 200, 30)
			owner := createTestUser(t, store, 51_020)
			other := createTestUser(t, store, 51_021)
			var ownerPurchaseID string
			for _, userID := range []string{owner.ID, other.ID} {
				if _, err := store.AdjustBalance(ctx, userID, 1_000, "stock-seed:"+userID, "seed", now); err != nil {
					t.Fatal(err)
				}
				purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: userID, ComboID: combo.ID,
					AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "stock-source"}, now)
				if err != nil {
					t.Fatal(err)
				}
				if userID == owner.ID {
					ownerPurchaseID = purchase.ID
				}
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status=? WHERE id=?`, test.status, ownerPurchaseID); err != nil {
				t.Fatal(err)
			}
			if _, err := store.SaveSquadProduct(ctx, SquadProductInput{GroupID: addon.ID, Name: addon.Name,
				UpstreamPresent: true, Visible: true, PriceTXBMinor: 400, StockLimit: &test.limit}); err != nil {
				t.Fatal(err)
			}
			quote, err := store.QuotePurchase(ctx, PurchaseInput{UserID: owner.ID, ComboID: combo.ID, AddonSquadIDs: []string{addon.ID}}, now)
			if test.held {
				if err != nil || quote.NetPriceTXBMinor != 600 {
					t.Fatalf("QuotePurchase(held seat) = (%+v, %v), want current 600", quote, err)
				}
			} else if !errors.Is(err, ErrStockUnavailable) {
				t.Fatalf("QuotePurchase(released seat) = %v, want ErrStockUnavailable", err)
			}
			newcomer := createTestUser(t, store, 51_022)
			if _, err := store.QuotePurchase(ctx, PurchaseInput{UserID: newcomer.ID, ComboID: combo.ID, AddonSquadIDs: []string{addon.ID}}, now); !errors.Is(err, ErrStockUnavailable) {
				t.Fatalf("QuotePurchase(new member) = %v, want ErrStockUnavailable", err)
			}
		})
	}
}
