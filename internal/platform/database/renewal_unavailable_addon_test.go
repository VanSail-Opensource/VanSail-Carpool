package database

import (
	"context"
	"testing"
	"time"
)

func TestRenewalSelectionExcludesUnavailableAddons(t *testing.T) {
	for _, automatic := range []bool{false, true} {
		name := "manual"
		if automatic {
			name = "automatic"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			store := newTestStore(t)
			user := createTestUser(t, store, 52_000)
			addon := saveTestSquad(t, store, "removed-addon", 300, true)
			combo := saveTestCombo(t, store, "core-only-renewal", 1_000, 30)
			now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
			if _, err := store.AdjustBalance(ctx, user.ID, 10_000, name+"-seed", "seed", now); err != nil {
				t.Fatal(err)
			}
			source, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, AddonSquadIDs: []string{addon.ID}, IdempotencyKey: name + "-source"}, now)
			if err != nil {
				t.Fatal(err)
			}
			var renewedID string
			if automatic {
				if err := store.SetAutoRenewal(ctx, user.ID, source.ID, true, now); err != nil {
					t.Fatalf("SetAutoRenewal(on): %v", err)
				}

				purchase, renewErr := store.CommitAutoRenewalExcludingAddons(ctx, source.ID, []string{addon.ID}, source.ValidUntil)
				if renewErr != nil {
					t.Fatal(renewErr)
				}
				renewedID = purchase.ID
			} else {
				quote, quoteErr := store.RenewalQuoteExcludingAddons(ctx, user.ID, source.ID, 1, []string{addon.ID}, now)
				if quoteErr != nil || quote.PricePerTerm.MinorInt64() != 1_000 || len(quote.AddonSquadUUIDs) != 0 {
					t.Fatalf("RenewalQuoteExcludingAddons() = (%+v, %v)", quote, quoteErr)
				}
				batch, renewErr := store.RenewExcludingAddons(ctx, RenewalInput{UserID: user.ID, PurchaseID: source.ID, TermCount: 1, IdempotencyKey: name + "-renewal"}, []string{addon.ID}, now)
				if renewErr != nil || len(batch.Purchases) != 1 {
					t.Fatalf("RenewExcludingAddons() = (%+v, %v)", batch, renewErr)
				}
				renewedID = batch.Purchases[0].ID
			}
			var price, addons int64
			if err := store.DB().QueryRowContext(ctx, `SELECT charged_txb_minor,(SELECT COUNT(*) FROM purchase_addons WHERE purchase_id=?) FROM purchases WHERE id=?`, renewedID, renewedID).Scan(&price, &addons); err != nil {
				t.Fatal(err)
			}
			if price != 1_000 || addons != 0 {
				t.Fatalf("renewed purchase price/addons = %d/%d, want 1000/0", price, addons)
			}
		})
	}
}
