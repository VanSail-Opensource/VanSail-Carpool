package database

import (
	"context"
	"errors"
	"testing"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestEditAdminEntitlementUsesUpdatedAtAndPreservesPricingFacts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	actor := createTestUser(t, store, 41001)
	originalCombo := saveTestCombo(t, store, "original", 1_250, 30)
	replacementCombo := saveTestCombo(t, store, "replacement", 9_900, 30)
	user, purchase := createAdminWorkflowPurchase(t, store, 41002, originalCombo, now)

	updatedAt := now.Add(time.Minute)
	input := AdminEntitlementEditInput{ActorUserID: actor.ID, UserID: user.ID, PurchaseID: purchase.ID,
		IdempotencyKey: "edit-entitlement", RequestFingerprint: "edit-fingerprint-0001", Reason: "correct entitlement",
		ExpectedUpdatedAt: purchase.UpdatedAt, ComboID: replacementCombo.ID, Status: "active",
		ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(45 * 24 * time.Hour), TrafficLimitBytes: 8_192,
		ResetStrategy: "DAY", SquadUUIDs: []string{}}
	updated, err := store.EditAdminEntitlement(ctx, input, updatedAt)
	if err != nil {
		t.Fatalf("EditAdminEntitlement(): %v", err)
	}
	if updated.ComboID != replacementCombo.ID || !updated.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("updated entitlement = %+v", updated)
	}
	if updated.PriceTXBMinor != purchase.PriceTXBMinor || updated.GrossPriceTXBMinor != purchase.GrossPriceTXBMinor ||
		updated.CoreGrossTXBMinor != purchase.CoreGrossTXBMinor || !updated.CreatedAt.Equal(purchase.CreatedAt) {
		t.Fatalf("immutable purchase facts changed: before=%+v after=%+v", purchase, updated)
	}
	if err := store.SetAutoRenewal(ctx, user.ID, updated.ID, true, updatedAt); err != nil {
		t.Fatalf("SetAutoRenewal(on): %v", err)
	}
	recordAutomaticRollover(t, store, updated.ID, updated.ValidUntil, 1_000, 0)
	successor, err := store.CommitAutoRenewal(ctx, updated.ID, updated.ValidUntil)
	if err != nil {
		t.Fatalf("CommitAutoRenewal(): %v", err)
	}
	// Edited hours extend only the current term; the renewal returns to the combo cadence.
	if want := updated.ValidUntil.AddDate(0, 0, replacementCombo.ValidityDays); !successor.ValidUntil.Equal(want) ||
		successor.TrafficLimitBytes != input.TrafficLimitBytes || successor.ResetStrategy != input.ResetStrategy {
		t.Fatalf("renewal did not preserve edited entitlement: %+v", successor)
	}
	if err := store.EnqueueDueEntitlementTransitions(ctx, updated.ValidUntil); err != nil {
		t.Fatalf("EnqueueDueEntitlementTransitions(): %v", err)
	}
	var rolloverLimit int64
	if err := store.DB().QueryRowContext(ctx, `SELECT traffic_limit_bytes FROM purchase_rollovers WHERE purchase_id=?`, updated.ID).Scan(&rolloverLimit); err != nil {
		t.Fatal(err)
	}
	if rolloverLimit != input.TrafficLimitBytes {
		t.Fatalf("rollover traffic limit = %d, want %d", rolloverLimit, input.TrafficLimitBytes)
	}
	var notificationKind string
	var pending int
	if err := store.DB().QueryRowContext(ctx, `SELECT kind,queued_at IS NULL FROM user_notification_events WHERE user_id=?`,
		user.ID).Scan(&notificationKind, &pending); err != nil {
		t.Fatal(err)
	}
	if notificationKind != jobpayload.UserEventAdminExtension || pending != 1 {
		t.Fatalf("extension notification = %q pending %d", notificationKind, pending)
	}
	assertNotificationCounts(t, store, 1, 0)

	input.IdempotencyKey = "stale-edit"
	input.RequestFingerprint = "edit-fingerprint-0002"
	if _, err := store.EditAdminEntitlement(ctx, input, updatedAt.Add(time.Minute)); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale edit error = %v, want ErrConflict", err)
	}
}

func TestEditQueuedAdminEntitlementReconcilesContinuitySchedule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 18, 8, 30, 0, 0, time.UTC)
	combo := saveTestCombo(t, store, "queued-edit", 1_250, 30)
	actor, _ := createAdminWorkflowPurchase(t, store, 41_101, combo, now)
	user, _ := createAdminWorkflowPurchase(t, store, 41_102, combo, now)
	queued, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID,
		IdempotencyKey: "queued-edit-successor"}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("CreatePurchase(queued): %v", err)
	}
	shiftedFrom := queued.ValidFrom.Add(48 * time.Hour)
	shiftedUntil := queued.ValidUntil.Add(48 * time.Hour)
	updated, err := store.EditAdminEntitlement(ctx, AdminEntitlementEditInput{ActorUserID: actor.ID, UserID: user.ID,
		PurchaseID: queued.ID, IdempotencyKey: "queued-edit-shift", RequestFingerprint: "queued-edit-shift-fingerprint",
		Reason: "move queued term", ExpectedUpdatedAt: queued.UpdatedAt, ComboID: combo.ID, Status: "queued",
		ValidFrom: shiftedFrom, ValidUntil: shiftedUntil, TrafficLimitBytes: queued.TrafficLimitBytes,
		ResetStrategy: queued.ResetStrategy, SquadUUIDs: queued.SquadUUIDs}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("EditAdminEntitlement(shift): %v", err)
	}
	var availableValue string
	if err := store.DB().QueryRowContext(ctx, `SELECT available_at FROM outbox_jobs WHERE kind=? AND payload=? AND status='pending'`,
		jobpayload.ContinuityKind, `{"purchaseId":"`+queued.ID+`"}`).Scan(&availableValue); err != nil {
		t.Fatalf("load shifted continuity job: %v", err)
	}
	available, err := parseStamp(availableValue)
	if err != nil || !available.Equal(shiftedFrom.Add(-EntitlementContinuityLead)) {
		t.Fatalf("continuity available_at = %s, %v", available, err)
	}
	_, err = store.EditAdminEntitlement(ctx, AdminEntitlementEditInput{ActorUserID: actor.ID, UserID: user.ID,
		PurchaseID: queued.ID, IdempotencyKey: "queued-edit-cancel", RequestFingerprint: "queued-edit-cancel-fingerprint",
		Reason: "cancel queued term", ExpectedUpdatedAt: updated.UpdatedAt, ComboID: combo.ID, Status: "cancelled",
		ValidFrom: shiftedFrom, ValidUntil: shiftedUntil, TrafficLimitBytes: queued.TrafficLimitBytes,
		ResetStrategy: queued.ResetStrategy, SquadUUIDs: queued.SquadUUIDs}, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("EditAdminEntitlement(cancel): %v", err)
	}
	var continuityJobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=?
		AND json_extract(payload,'$.purchaseId')=?`, jobpayload.ContinuityKind, queued.ID).Scan(&continuityJobs); err != nil {
		t.Fatal(err)
	}
	if continuityJobs != 0 {
		t.Fatalf("continuity jobs after cancellation = %d, want 0", continuityJobs)
	}
}

func TestRefundAdminEntitlementCreditsOriginalDebitExactlyOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC)
	actor := createTestUser(t, store, 41003)
	combo := saveTestCombo(t, store, "refundable", 1_700, 30)
	user, purchase := createAdminWorkflowPurchase(t, store, 41004, combo, now)
	before := adminWorkflowBalance(t, store, user.ID)
	input := AdminEntitlementRefundInput{ActorUserID: actor.ID, UserID: user.ID, PurchaseID: purchase.ID,
		IdempotencyKey: "refund-entitlement", RequestFingerprint: "refund-fingerprint-01", Reason: "approved refund", AmountTXBMinor: purchase.PriceTXBMinor}

	first, err := store.RefundAdminEntitlement(ctx, input, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("RefundAdminEntitlement(first): %v", err)
	}
	second, err := store.RefundAdminEntitlement(ctx, input, now.Add(2*time.Minute))
	if err != nil || second.ID != first.ID {
		t.Fatalf("RefundAdminEntitlement(replay) = %+v, %v", second, err)
	}
	if got := adminWorkflowBalance(t, store, user.ID); got != before+purchase.PriceTXBMinor {
		t.Fatalf("balance = %d, want %d", got, before+purchase.PriceTXBMinor)
	}
	if got := adminWorkflowLedgerCount(t, store, user.ID, "admin_entitlement_refund"); got != 1 {
		t.Fatalf("refund ledger count = %d, want 1", got)
	}
	assertNotificationCounts(t, store, 1, 0)
	refunded, err := store.PurchaseByID(ctx, purchase.ID)
	if err != nil || refunded.Status != "cancelled" {
		t.Fatalf("refunded purchase = %+v, %v", refunded, err)
	}
}

func TestReplaceAdminComboMovesNoTXB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	actor := createTestUser(t, store, 41005)
	paidAddon := saveTestSquad(t, store, "11111111-1111-4111-8111-111111111111", 300, true)
	newIncluded := saveTestSquad(t, store, "22222222-2222-4222-8222-222222222222", 0, true)
	original := saveTestCombo(t, store, "original-combo", 2_000, 30)
	replacement := saveTestCombo(t, store, "replacement-combo", 8_000, 30, newIncluded.ID)
	user, purchase := createAdminWorkflowPurchase(t, store, 41006, original, now, paidAddon.RemnaSquadUUID)
	beforeBalance := adminWorkflowBalance(t, store, user.ID)
	beforeLedger, err := store.ListLedger(ctx, user.ID, 100)
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.ReplaceAdminCombo(ctx, AdminComboReplacementInput{ActorUserID: actor.ID, UserID: user.ID,
		ComboID: replacement.ID, IdempotencyKey: "replace-combo", RequestFingerprint: "replace-fingerprint-01",
		Reason: "approved replacement", AddonSquadUUIDs: []string{"33333333-3333-4333-8333-333333333333"}}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ReplaceAdminCombo(): %v", err)
	}
	after, err := store.PurchaseByID(ctx, purchase.ID)
	if err != nil {
		t.Fatal(err)
	}
	afterLedger, err := store.ListLedger(ctx, user.ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if after.ComboID != replacement.ID || after.CoreGrossTXBMinor != purchase.CoreGrossTXBMinor {
		t.Fatalf("replacement purchase = %+v", after)
	}
	if adminWorkflowBalance(t, store, user.ID) != beforeBalance || len(afterLedger) != len(beforeLedger) {
		t.Fatal("no-charge replacement moved TXB")
	}
	if len(after.SquadUUIDs) != 2 {
		t.Fatalf("replacement squads = %v, want included plus override add-on", after.SquadUUIDs)
	}
}
