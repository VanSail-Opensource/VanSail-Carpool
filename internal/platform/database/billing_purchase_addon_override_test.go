package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPurchaseAddonOverrideRejectsWithoutFinancialWrites(t *testing.T) {
	for _, override := range []struct {
		name        string
		core, addon any
	}{{"core_empty", "[]", nil}, {"addon_empty", nil, "[]"}, {"core_selection", `["manual"]`, nil}, {"addon_selection", nil, `["manual"]`}, {"both_empty", "[]", "[]"}} {
		t.Run(override.name, func(t *testing.T) {
			ctx, store := context.Background(), newTestStore(t)
			user := createTestUser(t, store, 69010)
			now := time.Now().UTC()
			combo := saveTestCombo(t, store, "override-combo", 1000, 30)
			addon := saveTestSquad(t, store, "override-addon", 300, true)
			if _, err := store.AdjustBalance(ctx, user.ID, 5000, "seed", "seed", now); err != nil {
				t.Fatal(err)
			}
			purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "base"}, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.DB().ExecContext(ctx, `UPDATE purchases SET entitlement_squad_uuids=?,entitlement_addon_squad_uuids=? WHERE id=?`, override.core, override.addon, purchase.ID); err != nil {
				t.Fatal(err)
			}
			before := addonFinancialState(t, store, user.ID)
			input := PurchaseAddonInput{UserID: user.ID, PurchaseID: purchase.ID, AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "blocked"}
			if _, err := store.QuotePurchaseAddons(ctx, input, now); !errors.Is(err, ErrAddonEntitlementOverride) {
				t.Fatal(err)
			}
			if _, err := store.AddPurchaseAddons(ctx, input, now); !errors.Is(err, ErrAddonEntitlementOverride) {
				t.Fatal(err)
			}
			if after := addonFinancialState(t, store, user.ID); after != before {
				t.Fatal("override changed financial facts", before, after)
			}
		})
	}
}

func addonFinancialState(t *testing.T, store *Store, userID string) [4]int64 {
	t.Helper()
	var result [4]int64
	for i, query := range []string{`SELECT txb_minor FROM balances WHERE user_id=?`, `SELECT COUNT(*) FROM ledger_entries WHERE user_id=?`, `SELECT COUNT(*) FROM purchase_addon_adjustments WHERE purchase_id IN (SELECT id FROM purchases WHERE user_id=?)`, `SELECT COUNT(*) FROM purchase_addons WHERE purchase_id IN (SELECT id FROM purchases WHERE user_id=?)`} {
		if err := store.DB().QueryRow(query, userID).Scan(&result[i]); err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func TestPurchaseAddonDuplicateAndChargedReplaySurviveOverride(t *testing.T) {
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 69011)
	now := time.Now().UTC()
	combo := saveTestCombo(t, store, "duplicate-combo", 1000, 30)
	addon := saveTestSquad(t, store, "duplicate-addon", 300, true)
	if _, err := store.AdjustBalance(ctx, user.ID, 5000, "seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "base"}, now)
	if err != nil {
		t.Fatal(err)
	}
	input := PurchaseAddonInput{UserID: user.ID, PurchaseID: purchase.ID, AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "paid"}
	if _, err := store.AddPurchaseAddons(ctx, input, now); err != nil {
		t.Fatal(err)
	}
	before := addonFinancialState(t, store, user.ID)
	input.IdempotencyKey = "duplicate"
	if _, err := store.AddPurchaseAddons(ctx, input, now); !errors.Is(err, ErrSquadAlreadyAdded) {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`UPDATE purchases SET entitlement_addon_squad_uuids='[]' WHERE id=?`, purchase.ID); err != nil {
		t.Fatal(err)
	}
	input.IdempotencyKey = "paid"
	if _, err := store.AddPurchaseAddons(ctx, input, now); err != nil {
		t.Fatal("charged replay blocked", err)
	}
	if after := addonFinancialState(t, store, user.ID); after != before {
		t.Fatal("replay charged again", before, after)
	}
}
