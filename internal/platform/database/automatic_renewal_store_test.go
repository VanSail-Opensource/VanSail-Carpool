package database

import (
	"context"
	"errors"
	"testing"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestAutomaticRenewalCommitsAfterCalculatedRollover(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 48_000)
	combo := saveTestCombo(t, store, "automatic-continuity", 100, 30)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "automatic-continuity-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	source, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "automatic-continuity"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetAutoRenewal(ctx, user.ID, source.ID, true, now); err != nil {
		t.Fatalf("SetAutoRenewal(on): %v", err)
	}
	tooEarly := source.ValidUntil.Add(-EntitlementContinuityLead - time.Second)
	if due, err := store.DueAutoRenewals(ctx, tooEarly); err != nil || len(due) != 0 {
		t.Fatalf("DueAutoRenewals(early) = (%v, %v)", due, err)
	}
	if _, err := store.CommitAutoRenewal(ctx, source.ID, tooEarly); !errors.Is(err, ErrConflict) {
		t.Fatalf("CommitAutoRenewal(early) = %v, want ErrConflict", err)
	}
	if due, err := store.DueAutoRenewals(ctx, source.ValidUntil); err != nil || len(due) != 0 {
		t.Fatalf("DueAutoRenewals(before calculation) = (%v, %v)", due, err)
	}
	recordAutomaticRollover(t, store, source.ID, source.ValidUntil, 500, 500)
	if due, err := store.DueAutoRenewals(ctx, source.ValidUntil); err != nil || len(due) != 1 || due[0].PurchaseID != source.ID {
		t.Fatalf("DueAutoRenewals(calculated) = (%v, %v)", due, err)
	}
	successor, err := store.CommitAutoRenewal(ctx, source.ID, source.ValidUntil)
	if err != nil {
		t.Fatalf("CommitAutoRenewal(calculated): %v", err)
	}
	var applyJobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind='remna_apply_entitlement' AND payload=?`,
		`{"purchaseId":"`+successor.ID+`"}`).Scan(&applyJobs); err != nil || applyJobs != 1 {
		t.Fatalf("automatic activation jobs = %d, %v", applyJobs, err)
	}
	if err := store.MarkPurchaseSyncResult(ctx, successor.ID, true, source.ValidUntil); err != nil {
		t.Fatal(err)
	}
	var kind, used, eligible string
	if err := store.DB().QueryRowContext(ctx, `SELECT kind,json_extract(payload_json,'$.facts.usedBytes'),
		json_extract(payload_json,'$.facts.eligibleBytes') FROM user_notification_events WHERE event_key=?`,
		"auto-renewal:"+successor.ID).Scan(&kind, &used, &eligible); err != nil {
		t.Fatal(err)
	}
	if kind != jobpayload.UserEventAutoRenewal || used != "500" || eligible != "500" {
		t.Fatalf("automatic renewal notification = %q used %q eligible %q", kind, used, eligible)
	}
	assertNotificationCounts(t, store, 1, 1)
}

func TestAutomaticRenewalDefaultsToggleAndSuccessorIdempotency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 48_001)
	combo := saveTestCombo(t, store, "automatic", 100, 30)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "automatic-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	first, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "automatic-first"}, now)
	if err != nil || first.AutoRenewEnabled {
		t.Fatalf("CreatePurchase(immediate) = (%+v, %v), want auto renewal disabled", first, err)
	}
	if err := store.SetAutoRenewal(ctx, user.ID, first.ID, false, now); err != nil {
		t.Fatalf("SetAutoRenewal(off): %v", err)
	}
	if err := store.SetAutoRenewal(ctx, user.ID, first.ID, true, now); err != nil {
		t.Fatalf("SetAutoRenewal(on): %v", err)
	}
	recordAutomaticRollover(t, store, first.ID, first.ValidUntil, 1_000, 0)
	successor, err := store.CommitAutoRenewal(ctx, first.ID, first.ValidUntil)
	if err != nil || successor.Status != "activating" || !successor.AutoRenewEnabled || successor.CoreGrossTXBMinor != combo.PriceTXBMinor {
		t.Fatalf("CommitAutoRenewal() = (%+v, %v)", successor, err)
	}
	replayed, err := store.CommitAutoRenewal(ctx, first.ID, first.ValidUntil.Add(time.Second))
	if err != nil || replayed.ID != successor.ID {
		t.Fatalf("CommitAutoRenewal(replay) = (%+v, %v), want %s", replayed, err, successor.ID)
	}
	if due, err := store.DueAutoRenewals(ctx, first.ValidUntil.Add(time.Second)); err != nil || len(due) != 0 {
		t.Fatalf("DueAutoRenewals(after successor) = (%+v, %v), want none", due, err)
	}
	var successors, debits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM purchases WHERE auto_renew_source_purchase_id=?`, first.ID).Scan(&successors); err != nil {
		t.Fatal(err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger_entries WHERE kind='automatic_renewal'`).Scan(&debits); err != nil {
		t.Fatal(err)
	}
	if successors != 1 || debits != 1 {
		t.Fatalf("automatic renewal effects = successors:%d debits:%d, want 1/1", successors, debits)
	}

	other := createTestUser(t, store, 48_002)
	if _, err := store.AdjustBalance(ctx, other.ID, 500, "queued-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreatePurchase(ctx, PurchaseInput{UserID: other.ID, ComboID: combo.ID, IdempotencyKey: "queued-first"}, now); err != nil {
		t.Fatal(err)
	}
	queued, err := store.CreatePurchase(ctx, PurchaseInput{UserID: other.ID, ComboID: combo.ID, IdempotencyKey: "queued-next"}, now.Add(time.Hour))
	if err != nil || queued.AutoRenewEnabled {
		t.Fatalf("CreatePurchase(queued) = (%+v, %v), want auto renewal disabled", queued, err)
	}
}

func TestAutomaticRenewalFailureDisablesWithoutCharging(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 48_003)
	combo := saveTestCombo(t, store, "automatic-failure", 100, 30)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 100, "failure-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	source, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "failure-source"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetAutoRenewal(ctx, user.ID, source.ID, true, now); err != nil {
		t.Fatalf("SetAutoRenewal(on): %v", err)
	}
	recordAutomaticRollover(t, store, source.ID, source.ValidUntil, 1_000, 0)
	if _, err := store.CommitAutoRenewal(ctx, source.ID, source.ValidUntil); !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("CommitAutoRenewal(insufficient) = %v, want ErrInsufficientBalance", err)
	}
	if err := store.MarkAutoRenewalFailed(ctx, source.ID, AutoRenewalReasonInsufficientBalance, source.ValidUntil); err != nil {
		t.Fatal(err)
	}
	updated, err := store.PurchaseByID(ctx, source.ID)
	if err != nil || updated.AutoRenewEnabled {
		t.Fatalf("PurchaseByID(failed) = (%+v, %v), want disabled", updated, err)
	}
	failure, err := store.AutoRenewalFailure(ctx, user.ID)
	if err != nil || failure == nil || failure.Reason != AutoRenewalReasonInsufficientBalance {
		t.Fatalf("AutoRenewalFailure() = (%+v, %v)", failure, err)
	}
	if balance, err := store.Balance(ctx, user.ID); err != nil || balance.Minor != "0" {
		t.Fatalf("Balance(after failed renewal) = (%+v, %v), want 0", balance, err)
	}
	if err := store.EnqueueDueEntitlementTransitions(ctx, source.ValidUntil); err != nil {
		t.Fatalf("EnqueueDueEntitlementTransitions(): %v", err)
	}
	var rollovers int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM purchase_rollovers WHERE purchase_id=?`, source.ID).Scan(&rollovers); err != nil {
		t.Fatal(err)
	}
	if rollovers != 1 {
		t.Fatalf("rollovers after failure = %d, want 1", rollovers)
	}
}

func TestRenewalAwaitingRolloverTracksSettlement(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 48_004)
	combo := saveTestCombo(t, store, "rollover-awaiting", 100, 30)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 100, "awaiting-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	source, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "awaiting-source"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if awaiting, err := store.RenewalAwaitingRollover(ctx, source.ID); err != nil || !awaiting {
		t.Fatalf("RenewalAwaitingRollover(active) = (%t, %v), want true", awaiting, err)
	}
	if awaiting, err := store.RenewalAwaitingRollover(ctx, "missing-purchase"); err != nil || awaiting {
		t.Fatalf("RenewalAwaitingRollover(unknown) = (%t, %v), want false", awaiting, err)
	}
	if err := store.EnqueueDueEntitlementTransitions(ctx, source.ValidUntil); err != nil {
		t.Fatalf("EnqueueDueEntitlementTransitions(): %v", err)
	}
	if awaiting, err := store.RenewalAwaitingRollover(ctx, source.ID); err != nil || !awaiting {
		t.Fatalf("RenewalAwaitingRollover(pending) = (%t, %v), want true", awaiting, err)
	}
	if err := store.MarkRolloverProcessing(ctx, source.ID, source.ValidUntil); err != nil {
		t.Fatal(err)
	}
	if awaiting, err := store.RenewalAwaitingRollover(ctx, source.ID); err != nil || !awaiting {
		t.Fatalf("RenewalAwaitingRollover(processing) = (%t, %v), want true", awaiting, err)
	}
	if _, err := store.FinalizeRollover(ctx, source.ID, 100*1024*1024, 0, "", source.ValidUntil); err != nil {
		t.Fatal(err)
	}
	if awaiting, err := store.RenewalAwaitingRollover(ctx, source.ID); err != nil || awaiting {
		t.Fatalf("RenewalAwaitingRollover(settled) = (%t, %v), want false", awaiting, err)
	}
}
