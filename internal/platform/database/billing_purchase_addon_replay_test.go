package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestChargedAddonReplaySurvivesCatalogChangeWithoutAnotherDebit(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	saveTestSquad(t, store, "addon", 100, true)
	combo := saveTestCombo(t, store, "replay-native", 1000, 30)
	user, purchase := createAdminWorkflowPurchase(t, store, 89013, combo, now)
	input := PurchaseAddonInput{UserID: user.ID, PurchaseID: purchase.ID, AddonSquadIDs: []string{"addon"}, IdempotencyKey: "lost-response"}
	updated, err := store.AddPurchaseAddons(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	balance := adminWorkflowBalance(t, store, user.ID)
	saveTestSquad(t, store, "addon", 100, false)
	replayed, found, err := store.PurchaseAddonReplay(ctx, input)
	if err != nil || !found || replayed.ID != updated.ID || replayed.PriceTXBMinor != updated.PriceTXBMinor || adminWorkflowBalance(t, store, user.ID) != balance {
		t.Fatal("charged retry was not replayed", replayed, found, err)
	}
	input.AddonSquadIDs = []string{"different"}
	if _, _, err := store.PurchaseAddonReplay(ctx, input); !errors.Is(err, ErrConflict) {
		t.Fatal("reused key accepted different squads", err)
	}
}
