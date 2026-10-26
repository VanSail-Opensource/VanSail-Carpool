package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestRenewalsUseCurrentOptionalSquadPrices(t *testing.T) {
	t.Parallel()
	for _, automatic := range []bool{false, true} {
		for _, current := range []int64{700, 100, 0} {
			name := model.TXBMoney(current).Display
			if automatic {
				name = "automatic/" + name
			} else {
				name = "batch/" + name
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				ctx := context.Background()
				store := newTestStore(t)
				user := createTestUser(t, store, 51_001)
				addon := saveTestSquad(t, store, "repriced-addon", 300, true)
				combo := saveTestCombo(t, store, "repriced-renewal", 1_000, 30)
				now := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
				if _, err := store.AdjustBalance(ctx, user.ID, 10_000, "repricing-seed", "seed", now); err != nil {
					t.Fatal(err)
				}
				source, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID,
					AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "repricing-source"}, now)
				if err != nil {
					t.Fatal(err)
				}
				if err := store.SetAutoRenewal(ctx, user.ID, source.ID, false, now); err != nil {
					t.Fatal(err)
				}
				saveRenewalSquadPrice(t, store, addon.ID, current)
				plan, err := store.AutoRenewalPlan(ctx, user.ID, source.ID, now)
				if err != nil || plan.IneligibleReason != "" || plan.GrossMinor != 1_000+current || plan.NetMinor != 1_000+current {
					t.Fatalf("AutoRenewalPlan() = (%+v, %v), want current optional-squad price", plan, err)
				}
				quote, err := store.RenewalQuote(ctx, user.ID, source.ID, 2, now)
				if err != nil || quote.PricePerTerm.MinorInt64() != plan.NetMinor || quote.TotalPrice.MinorInt64() != 2*plan.NetMinor {
					t.Fatalf("RenewalQuote() = (%+v, %v), want current optional-squad price", quote, err)
				}
				// A later price edit must be picked up inside the debit transaction.
				chargedAddon := current + 50
				saveRenewalSquadPrice(t, store, addon.ID, chargedAddon)
				charged := 1_000 + chargedAddon
				var renewed []model.Purchase
				if automatic {
					if err := store.SetAutoRenewal(ctx, user.ID, source.ID, true, now); err != nil {
						t.Fatal(err)
					}

					successor, err := store.CommitAutoRenewal(ctx, source.ID, source.ValidUntil)
					if err != nil {
						t.Fatal(err)
					}
					renewed = []model.Purchase{successor}
					saveTestSquad(t, store, addon.ID, chargedAddon+100, true)
					replayed, err := store.CommitAutoRenewal(ctx, source.ID, source.ValidUntil)
					if err != nil || replayed.ID != successor.ID || replayed.PriceTXBMinor != charged {
						t.Fatalf("CommitAutoRenewal(replay) = (%+v, %v)", replayed, err)
					}
				} else {
					input := RenewalInput{UserID: user.ID, PurchaseID: source.ID, TermCount: 2, IdempotencyKey: "repricing-batch"}
					batch, err := store.Renew(ctx, input, now)
					if err != nil || len(batch.Purchases) != 2 || batch.TotalPrice.MinorInt64() != 2*charged {
						t.Fatalf("Renew() = (%+v, %v)", batch, err)
					}
					renewed = batch.Purchases
					saveTestSquad(t, store, addon.ID, chargedAddon+100, true)
					replayed, err := store.Renew(ctx, input, now)
					if err != nil || replayed.ID != batch.ID || replayed.TotalPrice != batch.TotalPrice {
						t.Fatalf("Renew(replay) = (%+v, %v)", replayed, err)
					}
					assertEventKind(t, store, "renewal-scheduled:"+batch.ID, jobpayload.UserEventRenewalScheduled)
				}
				for _, purchase := range renewed {
					if purchase.PriceTXBMinor != charged || purchase.GrossPriceTXBMinor != charged || purchase.CoreGrossTXBMinor != 1_000 {
						t.Fatalf("renewal price snapshot = %+v", purchase)
					}
					assertRenewalAddonCharge(t, store, purchase.ID, chargedAddon)
				}
				original, err := store.PurchaseByID(ctx, source.ID)
				if err != nil || original.PriceTXBMinor != 1_300 || original.GrossPriceTXBMinor != 1_300 {
					t.Fatalf("source price changed = (%+v, %v)", original, err)
				}
				assertRenewalAddonCharge(t, store, source.ID, 300)
				if balance, err := store.Balance(ctx, user.ID); err != nil || balance.MinorInt64() != 8_700-int64(len(renewed))*charged {
					t.Fatalf("Balance() = (%+v, %v), want one renewal debit", balance, err)
				}
			})
		}
	}
}

func saveRenewalSquadPrice(t *testing.T, store *Store, id string, current int64) {
	t.Helper()
	// Rising prices stay listed, lower prices become hidden, and zero prices
	// remove the sparse override entirely. All retain the purchased identity.
	if _, err := store.SaveSquadProduct(context.Background(), SquadProductInput{
		GroupID: id, Name: id, UpstreamPresent: true,
		PriceTXBMinor: current, Visible: current >= 300,
	}); err != nil {
		t.Fatal(err)
	}
}

func assertRenewalAddonCharge(t *testing.T, store *Store, purchaseID string, want int64) {
	t.Helper()
	var charged int64
	if err := store.DB().QueryRow(`SELECT charged_txb_minor FROM purchase_addons WHERE purchase_id=?`, purchaseID).Scan(&charged); err != nil {
		t.Fatal(err)
	}
	if charged != want {
		t.Fatalf("optional-squad charge = %d, want %d", charged, want)
	}
}
