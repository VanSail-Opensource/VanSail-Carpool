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
	if err := store.RefreshImportedSquads(ctx, []ImportedSquad{{UUID: "remote-1", Name: "Remote"}}); err != nil {
		t.Fatalf("RefreshImportedSquads(): %v", err)
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
	if err := store.RefreshImportedSquads(ctx, nil); err != nil {
		t.Fatalf("RefreshImportedSquads(empty): %v", err)
	}
	if combos, err := store.ListCombos(ctx, true); err != nil || len(combos) != 2 {
		t.Fatalf("ListCombos(after compatibility refresh) = (%+v, %v)", combos, err)
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
	if _, err := store.SaveHourlyOffer(ctx, user.ID, model.HourlyOffer{GroupID: "retained-a", Enabled: true, HourlyPriceTXBMinor: 1}, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveHourlyOffer(ctx, user.ID, model.HourlyOffer{GroupID: "retained-a", Enabled: false, HourlyPriceTXBMinor: 2}, 1); err != nil {
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
