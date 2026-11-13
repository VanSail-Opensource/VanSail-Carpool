package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPurchaseAndAddonReplaySurviveTerminalCompaction(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"expired", "cancelled", "failed"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			now := time.Now().UTC()
			old := now.Add(-90 * 24 * time.Hour)
			user := createTestUser(t, store, 497711)
			combo := saveTestCombo(t, store, "retained-commerce", 100, 30)
			addon := saveTestSquad(t, store, "retained-addon", 10, true)
			if _, err := store.AdjustBalance(ctx, user.ID, 1000, "retained-commerce-seed", "seed", old); err != nil {
				t.Fatal(err)
			}
			input := PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "original-checkout"}
			purchase, err := store.CreatePurchase(ctx, input, old)
			if err != nil {
				t.Fatal(err)
			}
			addition := PurchaseAddonInput{UserID: user.ID, PurchaseID: purchase.ID, AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "original-addition"}
			purchase, err = store.AddPurchaseAddons(ctx, addition, old.Add(time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			before, err := store.Balance(ctx, user.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status=$1,updated_at=$2 WHERE id=$3`, status, stamp(old), purchase.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE outbox_jobs SET status='done'`); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if _, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
					t.Fatal(err)
				}
				replayed, err := store.CreatePurchase(ctx, input, now)
				if err != nil || replayed.ID != purchase.ID || replayed.Status != status || replayed.PriceTXBMinor != purchase.PriceTXBMinor {
					t.Fatalf("terminal checkout replay changed: %+v, %v", replayed, err)
				}
				replayed, err = store.AddPurchaseAddons(ctx, addition, now)
				if err != nil || replayed.ID != purchase.ID || replayed.PriceTXBMinor != purchase.PriceTXBMinor {
					t.Fatalf("terminal addition replay changed: %+v, %v", replayed, err)
				}
				balance, err := store.Balance(ctx, user.ID)
				if err != nil || balance.Minor != before.Minor {
					t.Fatal("retained checkout replay charged again", balance, err)
				}
			}
			input.AddonSquadIDs = []string{addon.ID}
			if _, err := store.CreatePurchase(ctx, input, now); !errors.Is(err, ErrConflict) {
				t.Fatal("retained checkout key accepted changed intent", err)
			}
			addition.AddonSquadIDs = []string{"different-addon"}
			if _, err := store.AddPurchaseAddons(ctx, addition, now); !errors.Is(err, ErrConflict) {
				t.Fatal("retained adjustment key accepted changed intent", err)
			}
		})
	}
}

func TestLegacyPurchaseWithAddonReceiptSurvivesCompaction(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	old := now.Add(-90 * 24 * time.Hour)
	user := createTestUser(t, store, 497712)
	combo := saveTestCombo(t, store, "legacy-addon-receipt", 100, 30)
	addon := saveTestSquad(t, store, "legacy-addon-receipt-squad", 10, true)
	if _, err := store.AdjustBalance(ctx, user.ID, 1000, "legacy-addon-seed", "seed", old); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "legacy-fixture"}, old)
	if err != nil {
		t.Fatal(err)
	}
	input := PurchaseAddonInput{UserID: user.ID, PurchaseID: purchase.ID, AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "retained-adjustment"}
	if _, err := store.AddPurchaseAddons(ctx, input, old.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET idempotency_key=NULL,status='expired',updated_at=$1 WHERE id=$2`, stamp(old), purchase.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE outbox_jobs SET status='done'`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if replay, found, err := store.PurchaseAddonReplay(ctx, input); err != nil || !found || replay.ID != purchase.ID {
		t.Fatal("legacy purchase lost its charged adjustment receipt", replay, found, err)
	}
}

func TestRenewalBatchReplayRetainsEveryCompactedTerm(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	old := now.Add(-120 * 24 * time.Hour)
	user := createTestUser(t, store, 497713)
	combo := saveTestCombo(t, store, "retained-renewal", 100, 30)
	if _, err := store.AdjustBalance(ctx, user.ID, 1000, "renewal-replay-seed", "seed", old); err != nil {
		t.Fatal(err)
	}
	source, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "source"}, old)
	if err != nil {
		t.Fatal(err)
	}
	input := RenewalInput{UserID: user.ID, PurchaseID: source.ID, TermCount: 2, IdempotencyKey: "renewal"}
	batch, err := store.Renew(ctx, input, old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='expired',updated_at=$1 WHERE user_id=$2`, stamp(old), user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE outbox_jobs SET status='done'`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
			t.Fatal(err)
		}
		replay, err := store.Renew(ctx, input, now)
		if err != nil || replay.ID != batch.ID || len(replay.Purchases) != len(batch.Purchases) {
			t.Fatal("renewal replay lost retained terms", replay, err)
		}
		for index := range batch.Purchases {
			if replay.Purchases[index].ID != batch.Purchases[index].ID {
				t.Fatal("renewal replay replaced a retained term")
			}
		}
		balance, err := store.Balance(ctx, user.ID)
		if err != nil || balance.Minor != "700" {
			t.Fatal("renewal replay repeated debit", balance, err)
		}
	}
}
