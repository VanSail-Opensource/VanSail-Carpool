package database

import (
	"context"
	"errors"
	"testing"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
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
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='rollover_finalize' AND payload=?`, `{"purchaseId":"`+first.ID+`"}`).Scan(&count); err != nil {
		t.Fatalf("count rollover jobs: %v", err)
	}
	if count != 1 {
		t.Fatalf("rollover job count = %d, want 1", count)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_apply_entitlement' AND payload=?`, `{"purchaseId":"`+renewal.ID+`"}`).Scan(&count); err != nil {
		t.Fatalf("count renewal activation jobs: %v", err)
	}
	if count != 0 {
		t.Fatalf("renewal activated before rollover: count = %d", count)
	}
	if err := store.MarkRolloverProcessing(ctx, first.ID, first.ValidUntil); err != nil {
		t.Fatalf("MarkRolloverProcessing(): %v", err)
	}
	if _, err := store.FinalizeRollover(ctx, first.ID, 1000, 500, "", first.ValidUntil); err != nil {
		t.Fatalf("FinalizeRollover(): %v", err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_apply_entitlement' AND payload=?`, `{"purchaseId":"`+renewal.ID+`"}`).Scan(&count); err != nil {
		t.Fatalf("count renewal activation jobs after rollover: %v", err)
	}
	if count != 1 {
		t.Fatalf("renewal activation job count after rollover = %d, want 1", count)
	}
	if err := store.MarkPurchaseSyncResult(ctx, renewal.ID, true, first.ValidUntil); err != nil {
		t.Fatalf("MarkPurchaseSyncResult(renewal): %v", err)
	}
	var notificationKind string
	if err := store.DB().QueryRowContext(ctx, `SELECT kind FROM user_notification_events WHERE event_key=?`,
		"activation:"+renewal.ID).Scan(&notificationKind); err != nil {
		t.Fatal(err)
	}
	if notificationKind != jobpayload.UserEventQueuedActivation {
		t.Fatalf("manual successor notification = %q", notificationKind)
	}
	assertNotificationCounts(t, store, 2, 2)
}
