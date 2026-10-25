package database

import (
	"context"
	"testing"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestAdminBulkExtensionUsesInclusiveORDeduplicationAndQueueShift(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 18, 11, 0, 0, 0, time.UTC)
	comboA := saveTestCombo(t, store, "bulk-a", 100, 30)
	comboB := saveTestCombo(t, store, "bulk-b", 200, 30)
	addon := saveTestSquad(t, store, "44444444-4444-4444-8444-444444444444", 25, true)

	userOne, _ := createAdminWorkflowPurchase(t, store, 42001, comboA, now)
	second, err := store.CreatePurchase(ctx, PurchaseInput{UserID: userOne.ID, ComboID: comboA.ID, IdempotencyKey: "bulk-second"}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("CreatePurchase(second): %v", err)
	}
	third, err := store.CreatePurchase(ctx, PurchaseInput{UserID: userOne.ID, ComboID: comboA.ID, IdempotencyKey: "bulk-third"}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("CreatePurchase(third): %v", err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='active',valid_from=?,valid_until=?,updated_at=? WHERE id=?`,
		stamp(now.Add(-2*time.Hour)), stamp(now.Add(20*24*time.Hour)), stamp(now), second.ID); err != nil {
		t.Fatal(err)
	}
	second, err = store.PurchaseByID(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	userTwo, _ := createAdminWorkflowPurchase(t, store, 42002, comboB, now, addon.GroupID)
	actor, _ := createAdminWorkflowPurchase(t, store, 42003, comboB, now)

	filter := AdminBulkExtensionFilter{ComboIDs: []string{comboA.ID}, AddonSquadUUIDs: []string{addon.GroupID}}
	targets, preview, err := adminBulkTargets(ctx, store.db, filter, now)
	if err != nil {
		t.Fatalf("adminBulkTargets(): %v", err)
	}
	if preview.MatchedUsers != 2 || preview.ActivePurchases != 3 || preview.QueuedSuccessors != 1 || len(targets) != 2 {
		t.Fatalf("preview = %+v, targets = %+v", preview, targets)
	}
	targetIDs := map[string]bool{}
	for _, target := range targets {
		targetIDs[target.UserID] = true
	}
	if !targetIDs[userOne.ID] || !targetIDs[userTwo.ID] {
		t.Fatalf("targets = %+v", targets)
	}
	for _, target := range targets {
		if target.UserID == userOne.ID && target.PurchaseID != second.ID {
			t.Fatalf("user one target = %q, want current purchase %q", target.PurchaseID, second.ID)
		}
	}

	operation, err := store.CreateAdminBulkExtension(ctx, AdminBulkExtensionInput{ActorUserID: actor.ID,
		IdempotencyKey: "bulk-extension", RequestFingerprint: "bulk-extension-fingerprint", Reason: "service credit",
		Filter: filter, DurationMinutes: 5*24*60 + 17}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("CreateAdminBulkExtension(): %v", err)
	}
	if operation.Status != "queued" {
		t.Fatalf("operation status = %q, want queued", operation.Status)
	}
	shifted, err := store.PurchaseByID(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantShiftedUntil := second.ValidUntil.Add(5*24*time.Hour + 17*time.Minute)
	if !shifted.ValidUntil.Equal(wantShiftedUntil) {
		t.Fatalf("active expiry = %s, want %s", shifted.ValidUntil, wantShiftedUntil)
	}
	queued, err := store.PurchaseByID(ctx, third.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantQueuedFrom := third.ValidFrom.Add(5*24*time.Hour + 17*time.Minute)
	wantQueuedUntil := third.ValidUntil.Add(5*24*time.Hour + 17*time.Minute)
	if !queued.ValidFrom.Equal(wantQueuedFrom) || !queued.ValidUntil.Equal(wantQueuedUntil) {
		t.Fatalf("queued successor = %+v, want minute-precise shift", queued)
	}
	var retiredJobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=? AND payload=? AND status IN ('pending','processing')`,
		jobpayload.ContinuityKind, `{"purchaseId":"`+third.ID+`"}`).Scan(&retiredJobs); err != nil || retiredJobs != 0 {
		t.Fatal("purchase edit regenerated retired continuity work", retiredJobs, err)
	}
	var itemCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM provider_operation_items WHERE operation_id=?`, operation.ID).Scan(&itemCount); err != nil {
		t.Fatal(err)
	}
	if itemCount != 2 {
		t.Fatalf("bulk item count = %d, want 2", itemCount)
	}
}

func TestAdminBulkAddonFilterUsesEffectiveEntitlementOverrides(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 18, 15, 0, 0, 0, time.UTC)
	combo := saveTestCombo(t, store, "bulk-overrides", 100, 30)
	original := saveTestSquad(t, store, "55555555-5555-4555-8555-555555555555", 25, true)
	addonOverride := "66666666-6666-4666-8666-666666666666"
	fullOverride := "77777777-7777-4777-8777-777777777777"
	_, purchase := createAdminWorkflowPurchase(t, store, 42_004, combo, now, original.GroupID)
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET entitlement_addon_squad_uuids=? WHERE id=?`,
		`["`+addonOverride+`"]`, purchase.ID); err != nil {
		t.Fatal(err)
	}
	_, stalePreview, err := adminBulkTargets(ctx, store.db, AdminBulkExtensionFilter{AddonSquadUUIDs: []string{original.GroupID}}, now)
	if err != nil || stalePreview.MatchedUsers != 0 {
		t.Fatalf("stale add-on preview = %+v, %v", stalePreview, err)
	}
	_, addonPreview, err := adminBulkTargets(ctx, store.db, AdminBulkExtensionFilter{AddonSquadUUIDs: []string{addonOverride}}, now)
	if err != nil || addonPreview.MatchedUsers != 1 {
		t.Fatalf("add-on override preview = %+v, %v", addonPreview, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET entitlement_squad_uuids=? WHERE id=?`,
		`["`+fullOverride+`"]`, purchase.ID); err != nil {
		t.Fatal(err)
	}
	_, shadowedPreview, err := adminBulkTargets(ctx, store.db, AdminBulkExtensionFilter{AddonSquadUUIDs: []string{addonOverride}}, now)
	if err != nil || shadowedPreview.MatchedUsers != 0 {
		t.Fatalf("shadowed add-on override preview = %+v, %v", shadowedPreview, err)
	}
	_, fullPreview, err := adminBulkTargets(ctx, store.db, AdminBulkExtensionFilter{AddonSquadUUIDs: []string{fullOverride}}, now)
	if err != nil || fullPreview.MatchedUsers != 1 {
		t.Fatalf("full-squad override preview = %+v, %v", fullPreview, err)
	}
}
