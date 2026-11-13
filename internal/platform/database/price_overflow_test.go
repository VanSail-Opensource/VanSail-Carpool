package database

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func TestPurchasePricingRejectsOutOfRangeLegacyBasket(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 69_030)
	combo := saveTestCombo(t, store, "overflow-basket", 1<<62, 30)
	addons := []string{}
	for _, id := range []string{"overflow-a", "overflow-b", "overflow-c"} {
		addons = append(addons, saveTestSquad(t, store, id, 1<<62, true).ID)
	}
	input := PurchaseInput{UserID: user.ID, ComboID: combo.ID, AddonSquadIDs: addons, IdempotencyKey: "overflow-basket"}
	// Checkout's existing versioned pricing admission rejects this invalid
	// basket before the later sum guard; preserve its public error contract.
	if _, err := store.QuotePurchase(ctx, input, time.Now()); !errors.Is(err, pricing.VersionConflict) {
		t.Fatalf("overflowing basket produced a quote: %v", err)
	}
	if _, err := store.CreatePurchase(ctx, input, time.Now()); !errors.Is(err, pricing.VersionConflict) {
		t.Fatalf("overflowing basket created an entitlement: %v", err)
	}
}

func TestRenewalRejectsOverflowBeforeDebitOrSuccessor(t *testing.T) {
	t.Parallel()
	for _, automatic := range []bool{false, true} {
		name := "four_term_total"
		if automatic {
			name = "automatic_addon_sum"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			store := newTestStore(t)
			user := createTestUser(t, store, 69_031)
			now := time.Now().UTC()
			core := saveTestSquad(t, store, "overflow-core", 0, true)
			addon := saveTestSquad(t, store, "overflow-addon", 10, true)
			combo := saveTestCombo(t, store, "overflow-renewal", 100, 30, core.ID)
			if _, err := store.AdjustBalance(ctx, user.ID, 1_000, "overflow-seed", "fixture", now); err != nil {
				t.Fatal(err)
			}
			input := PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "overflow-source"}
			if automatic {
				input.AddonSquadIDs = []string{addon.ID}
			}
			purchase, err := store.CreatePurchase(ctx, input, now)
			if err != nil {
				t.Fatal(err)
			}
			before, err := store.Balance(ctx, user.ID)
			if err != nil {
				t.Fatal(err)
			}
			price := int64(1 << 62) // Four terms previously wrapped to a free total.
			if automatic {
				price = math.MaxInt64
				if err := store.SetAutoRenewal(ctx, user.ID, purchase.ID, true, now); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE combos SET price_txb_minor=$1 WHERE id=$2`, price, combo.ID); err != nil {
				t.Fatal(err)
			}
			if automatic {
				if _, err := store.AutoRenewalPlan(ctx, user.ID, purchase.ID, purchase.ValidUntil); !errors.Is(err, ErrConflict) {
					t.Fatalf("overflowing automatic plan accepted: %v", err)
				}
				if _, err := store.CommitAutoRenewal(ctx, purchase.ID, purchase.ValidUntil); !errors.Is(err, ErrConflict) {
					t.Fatalf("overflowing automatic renewal accepted: %v", err)
				}
			} else {
				if _, err := store.RenewalQuote(ctx, user.ID, purchase.ID, 4, now); !errors.Is(err, ErrConflict) {
					t.Fatalf("overflowing renewal quote accepted: %v", err)
				}
				if _, err := store.Renew(ctx, RenewalInput{UserID: user.ID, PurchaseID: purchase.ID, TermCount: 4, IdempotencyKey: "overflow-renewal"}, now); !errors.Is(err, ErrConflict) {
					t.Fatalf("overflowing renewal accepted: %v", err)
				}
			}
			after, err := store.Balance(ctx, user.ID)
			if err != nil || after.Minor != before.Minor {
				t.Fatal("rejected overflow changed balance", before.Minor, after.Minor, err)
			}
			var purchases int
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM purchases WHERE user_id=$1`, user.ID).Scan(&purchases); err != nil || purchases != 1 {
				t.Fatal("rejected overflow created successors", purchases, err)
			}
		})
	}
}
