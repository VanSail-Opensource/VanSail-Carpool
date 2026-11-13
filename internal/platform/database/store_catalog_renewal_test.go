package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestCatalogUpdatesCannotCreateUnknownRecords(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	comboInput := ComboInput{Name: "Known", PriceTXBMinor: 100, ValidityDays: 30, TrafficLimitBytes: 1024, ResetStrategy: "MONTH_ROLLING", Active: true}
	combo, err := store.SaveCombo(ctx, comboInput)
	if err != nil {
		t.Fatalf("SaveCombo(create): %v", err)
	}
	comboInput.ID = combo.ID
	comboInput.Name = "Updated"
	if updated, err := store.SaveCombo(ctx, comboInput); err != nil || updated.Name != "Updated" {
		t.Fatalf("SaveCombo(update) = (%+v, %v)", updated, err)
	}
	comboInput.ID = "client-selected-missing-id"
	if _, err := store.SaveCombo(ctx, comboInput); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SaveCombo(unknown update) = %v, want ErrNotFound", err)
	}

	if _, err := store.SaveSquadProduct(ctx, SquadProductInput{ID: "invented", GroupID: "invented", Name: "Invented"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SaveSquadProduct(unimported) = %v, want ErrNotFound", err)
	}
	if live, err := store.ImportSquad(ctx, "remote-1", "Remote"); err != nil || live.GroupID != "remote-1" || live.Name != "Remote" {
		t.Fatalf("ImportSquad(): %+v, %v", live, err)
	}
	if _, err := store.SquadProductByGroupID(ctx, "remote-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SquadProductByGroupID(unedited upstream) = %v, want ErrNotFound", err)
	}
	productInput := SquadProductInput{ID: "remote-1", GroupID: "remote-1", Name: "Merchandised", Description: "Local copy", PriceTXBMinor: 25, Visible: true, UpstreamPresent: true}
	product, err := store.SaveSquadProduct(ctx, productInput)
	if err != nil {
		t.Fatalf("SaveSquadProduct(imported create): %v", err)
	}
	if updated, err := store.SaveSquadProduct(ctx, productInput); err != nil || updated.Name != "Merchandised" || !updated.UpstreamPresent {
		t.Fatalf("SaveSquadProduct(imported update) = (%+v, %v)", updated, err)
	}
	comboInput.ID = ""
	comboInput.SquadProductIDs = []string{product.ID}
	squadCombo, err := store.SaveCombo(ctx, comboInput)
	if err != nil {
		t.Fatalf("SaveCombo(imported squad): %v", err)
	}
	if combos, err := store.ListCombos(ctx, true); err != nil || len(combos) != 2 {
		t.Fatalf("ListCombos(with live squad identity) = (%+v, %v)", combos, err)
	}
	if loaded, err := store.ComboByID(ctx, squadCombo.ID, true); err != nil || len(loaded.IncludedSquads) != 1 {
		t.Fatalf("ComboByID(sparse squad identity) = (%+v, %v)", loaded, err)
	}

	comboInput.ID = ""
	comboInput.SquadProductIDs = []string{"sparse-upstream-squad"}
	if _, err := store.SaveCombo(ctx, comboInput); err != nil {
		t.Fatalf("SaveCombo(sparse upstream identity): %v", err)
	}
}

func TestRenewalSelectionSurvivesBillingModeAndHourlyOfferEdits(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 20004)
	combo := saveTestCombo(t, store, "retained-renewal", 100, 30, "retained-a", "retained-b")
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "renewal-retain-seed", "fixture", time.Now()); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "retained-renewal"}, now)
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.AutoRenewalPlan(ctx, user.ID, purchase.ID, now)
	if err != nil || before.IneligibleReason != "" {
		t.Fatalf("AutoRenewalPlan(before edits) = (%+v, %v)", before, err)
	}
	if err := store.SaveXNodeGroup(ctx, xnode.Group{ID: "retained-a", Name: "Retained A", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveHourlyOffer(ctx, createTestAdmin(t, store, 9_897_000_001).ID, model.HourlyOffer{GroupID: "retained-a", Enabled: true, HourlyPriceTXBMinor: 1}, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveHourlyOffer(ctx, createTestAdmin(t, store, 9_897_000_001).ID, model.HourlyOffer{GroupID: "retained-a", Enabled: false, HourlyPriceTXBMinor: 2}, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveSquadProduct(ctx, SquadProductInput{GroupID: "retained-b", UpstreamPresent: true, BillingMode: "payg", PAYGPriceTXBMinor: 100}); err != nil {
		t.Fatal(err)
	}
	after, err := store.AutoRenewalPlan(ctx, user.ID, purchase.ID, now)
	if err != nil || after.IneligibleReason != "" {
		t.Fatalf("AutoRenewalPlan(after edits) = (%+v, %v)", after, err)
	}
	if !equalStrings(includedSquadIDs(after.Combo), []string{"retained-a", "retained-b"}) {
		t.Fatalf("automatic renewal selection changed: %v", includedSquadIDs(after.Combo))
	}
	if after.NetMinor != before.NetMinor {
		t.Fatalf("merchandising edits changed the renewal price: %d -> %d", before.NetMinor, after.NetMinor)
	}
	renewed, err := store.Renew(ctx, RenewalInput{UserID: user.ID, PurchaseID: purchase.ID, TermCount: 1, IdempotencyKey: "retained-renewal-commit"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(renewed.Purchases) != 1 || !equalStrings(renewed.Purchases[0].SquadUUIDs, []string{"retained-a", "retained-b"}) {
		t.Fatalf("committed renewal selection = %+v", renewed)
	}
}

func includedSquadIDs(combo model.Combo) []string {
	ids := make([]string, 0, len(combo.IncludedSquads))
	for _, squad := range combo.IncludedSquads {
		ids = append(ids, squad.GroupID)
	}
	return ids
}

// TestOwnedRenewalSurvivesHiddenComboAndBundleEdits retires (hides) an owned
// combo and then edits and reprices its live bundle. Both renewal entry points
// must still renew the purchased selection at the current price: the purchase
// snapshot keeps the sale-time squads, while the live bundle and the immutable
// purchase charge stay untouched. New sales and the storefront remain active-only.
func TestOwnedRenewalSurvivesHiddenComboAndBundleEdits(t *testing.T) {
	t.Parallel()
	for _, automatic := range []bool{false, true} {
		name := "batch"
		if automatic {
			name = "automatic"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			store := newTestStore(t)
			user := createTestUser(t, store, 51_010)
			core := saveTestSquad(t, store, "hidden-core", 100, true)
			retired := saveTestSquad(t, store, "hidden-excluded", 100, true)
			addon := saveTestSquad(t, store, "hidden-addon", 300, true)
			combo := saveTestCombo(t, store, "hidden-combo", 1_000, 30, core.ID, retired.ID)
			now := time.Now().UTC()
			if _, err := store.AdjustBalance(ctx, user.ID, 5_000, "hidden-combo-seed", "fixture", now); err != nil {
				t.Fatal(err)
			}
			// PAYG-marked included squads are excluded from the sale, so this
			// purchase records its diverged base selection in combo_squad_uuids.
			if _, err := store.SaveSquadProduct(ctx, SquadProductInput{GroupID: retired.ID, UpstreamPresent: true,
				BillingMode: "payg", PAYGPriceTXBMinor: 100}); err != nil {
				t.Fatal(err)
			}
			purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID,
				AddonSquadIDs: []string{addon.ID}, IdempotencyKey: "hidden-combo-source"}, now)
			if err != nil {
				t.Fatal(err)
			}
			var snapshot string
			if err := store.DB().QueryRowContext(ctx, `SELECT combo_squad_uuids FROM purchases WHERE id=$1`, purchase.ID).Scan(&snapshot); err != nil || snapshot != `["hidden-core"]` {
				t.Fatalf("purchase bundle snapshot = %q, %v", snapshot, err)
			}
			if err := store.DeleteCombo(ctx, combo.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE combos SET included_squad_uuids='["hidden-excluded"]',price_txb_minor=2000 WHERE id=$1`, combo.ID); err != nil {
				t.Fatal(err)
			}

			plan, err := store.AutoRenewalPlan(ctx, user.ID, purchase.ID, now)
			if err != nil || plan.IneligibleReason != "" || !equalStrings(includedSquadIDs(plan.Combo), []string{"hidden-core"}) ||
				len(plan.Addons) != 1 || plan.Addons[0].GroupID != addon.ID || plan.GrossMinor != 2_300 || plan.NetMinor != 2_300 {
				t.Fatalf("AutoRenewalPlan(hidden combo) = (%+v, %v), want purchased squads at the current 2300 price", plan, err)
			}
			quote, err := store.RenewalQuote(ctx, user.ID, purchase.ID, 1, now)
			if err != nil || quote.ComboID != combo.ID || quote.PricePerTerm.MinorInt64() != 2_300 || !equalStrings(quote.AddonSquadUUIDs, []string{addon.ID}) {
				t.Fatalf("RenewalQuote(hidden combo) = (%+v, %v)", quote, err)
			}
			owned, err := store.RenewalCombo(ctx, user.ID, purchase.ID)
			if err != nil || owned.ID != combo.ID || !equalStrings(includedSquadIDs(owned), []string{"hidden-core"}) {
				t.Fatalf("RenewalCombo(hidden combo) = (%+v, %v)", owned, err)
			}
			if _, err := store.RenewalCombo(ctx, "other-user", purchase.ID); !errors.Is(err, ErrNotFound) {
				t.Fatalf("RenewalCombo(foreign owner) = %v, want ErrNotFound", err)
			}
			if _, err := store.ComboByID(ctx, combo.ID, true); !errors.Is(err, ErrNotFound) {
				t.Fatalf("ComboByID(active only) = %v, want ErrNotFound", err)
			}
			if _, err := store.QuotePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID}, now); !errors.Is(err, ErrNotFound) {
				t.Fatalf("QuotePurchase(hidden combo) = %v, want ErrNotFound", err)
			}
			if _, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID,
				IdempotencyKey: "hidden-combo-new-sale"}, now); !errors.Is(err, ErrNotFound) {
				t.Fatalf("CreatePurchase(hidden combo) = %v, want ErrNotFound", err)
			}
			if _, err := store.RenewalCombo(ctx, user.ID, "missing-purchase"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("RenewalCombo(missing purchase) = %v, want ErrNotFound", err)
			}

			var successors []model.Purchase
			wantSourceStatus := "activating"
			if automatic {
				wantSourceStatus = "expired"
				if err := store.SetAutoRenewal(ctx, user.ID, purchase.ID, true, now); err != nil {
					t.Fatal(err)
				}
				successor, err := store.CommitAutoRenewal(ctx, purchase.ID, purchase.ValidUntil)
				if err != nil {
					t.Fatal(err)
				}
				successors = []model.Purchase{successor}
			} else {
				batch, err := store.Renew(ctx, RenewalInput{UserID: user.ID, PurchaseID: purchase.ID, TermCount: 1,
					IdempotencyKey: "hidden-combo-batch"}, purchase.ValidUntil)
				if err != nil {
					t.Fatal(err)
				}
				successors = batch.Purchases
			}
			for _, successor := range successors {
				// The paid successor keeps the purchased base selection in its own
				// combo snapshot, while its entitlement union also carries the paid
				// add-on. purchaseSquadsFrom merges both by design, so renewal must
				// never lose the add-on from the member's effective entitlements.
				var successorSnapshot string
				if err := store.DB().QueryRowContext(ctx, `SELECT combo_squad_uuids FROM purchases WHERE id=$1`, successor.ID).Scan(&successorSnapshot); err != nil || successorSnapshot != `["hidden-core"]` {
					t.Fatalf("renewal bundle snapshot = %q, %v", successorSnapshot, err)
				}
				if !equalStrings(successor.SquadUUIDs, []string{"hidden-addon", "hidden-core"}) || successor.ComboID != combo.ID {
					t.Fatalf("renewal entitlement union = (%+v)", successor)
				}
				if successor.PriceTXBMinor != 2_300 || successor.CoreGrossTXBMinor != 2_000 ||
					!successor.ValidFrom.Equal(purchase.ValidUntil) || !successor.ValidUntil.Equal(purchase.ValidUntil.AddDate(0, 0, 30)) {
					t.Fatalf("renewal pricing/dates = (%+v)", successor)
				}
				assertRenewalAddonCharge(t, store, successor.ID, 300)
			}
			if balance, err := store.Balance(ctx, user.ID); err != nil || balance.MinorInt64() != 1_400 {
				t.Fatalf("Balance() = (%+v, %v), want one renewal debit", balance, err)
			}
			if source, err := store.PurchaseByID(ctx, purchase.ID); err != nil || source.Status != wantSourceStatus || source.PriceTXBMinor != 1_300 {
				t.Fatalf("source purchase = (%+v, %v)", source, err)
			}
		})
	}
}

func TestDueRenewalWaitsForRolloverThenEnqueuesExactlyOneActivation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 20003)
	combo := saveTestCombo(t, store, "single-activation", 100, 30)
	if _, err := store.AdjustBalance(ctx, user.ID, 200, "activation-seed", "test credit", time.Now()); err != nil {
		t.Fatalf("AdjustBalance(): %v", err)
	}
	start := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	first, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "activation-first"}, start)
	if err != nil {
		t.Fatalf("CreatePurchase(first): %v", err)
	}
	renewal, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "activation-renewal"}, start.Add(time.Hour))
	if err != nil {
		t.Fatalf("CreatePurchase(renewal): %v", err)
	}
	if err := store.EnqueueDueEntitlementTransitions(ctx, first.ValidUntil); err != nil {
		t.Fatalf("EnqueueDueEntitlementTransitions(first): %v", err)
	}
	if err := store.EnqueueDueEntitlementTransitions(ctx, first.ValidUntil.Add(time.Second)); err != nil {
		t.Fatalf("EnqueueDueEntitlementTransitions(retry): %v", err)
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='purchase_expire' AND payload=$1`, `{"purchaseId":"`+first.ID+`"}`).Scan(&count); err != nil {
		t.Fatalf("count rollover jobs: %v", err)
	}
	if count != 1 {
		t.Fatalf("rollover job count = %d, want 1", count)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_apply_entitlement' AND payload=$1`, `{"purchaseId":"`+renewal.ID+`"}`).Scan(&count); err != nil {
		t.Fatalf("count renewal activation jobs: %v", err)
	}
	if count != 0 {
		t.Fatalf("renewal activated before rollover: count = %d", count)
	}
	if err := store.ExpireDuePurchase(ctx, first.ID, first.ValidUntil); err != nil {
		t.Fatalf("FinalizeRollover(): %v", err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_apply_entitlement' AND payload=$1`, `{"purchaseId":"`+renewal.ID+`"}`).Scan(&count); err != nil {
		t.Fatalf("count renewal activation jobs after rollover: %v", err)
	}
	if count != 1 {
		t.Fatalf("renewal activation job count after rollover = %d, want 1", count)
	}
	if err := store.MarkPurchaseSyncResult(ctx, renewal.ID, true, first.ValidUntil); err != nil {
		t.Fatalf("MarkPurchaseSyncResult(renewal): %v", err)
	}
	var notificationKind string
	if err := store.DB().QueryRowContext(ctx, `SELECT kind FROM user_notification_events WHERE event_key=$1`,
		"activation:"+renewal.ID).Scan(&notificationKind); err != nil {
		t.Fatal(err)
	}
	if notificationKind != jobpayload.UserEventQueuedActivation {
		t.Fatalf("manual successor notification = %q", notificationKind)
	}
	assertNotificationCounts(t, store, 2, 2)
}
