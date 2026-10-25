package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func TestTrafficResetDebitReplayAndCompensationAreAtomic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 28001)
	combo := saveTestCombo(t, store, "daily-reset", 301, 30)
	if _, err := store.DB().ExecContext(ctx, "UPDATE combos SET reset_strategy='DAY' WHERE id=?", combo.ID); err != nil {
		t.Fatalf("set daily strategy: %v", err)
	}
	if _, err := store.AdjustBalance(ctx, user.ID, 5_000, "reset-seed", "test credit", time.Now()); err != nil {
		t.Fatalf("AdjustBalance(): %v", err)
	}
	now := time.Date(2026, 8, 18, 1, 0, 0, 0, time.UTC)
	purchase := createMemberOperationPurchase(t, store, user.ID, combo.ID, "reset-purchase", now)
	if err := store.MarkPurchaseSyncResult(ctx, purchase.ID, true, now); err != nil {
		t.Fatalf("MarkPurchaseSyncResult(): %v", err)
	}
	input := providerops.CreateInput{ActorUserID: user.ID, OwnerUserID: user.ID, Kind: purchaseops.OperationResetKind,
		IdempotencyKey: "reset-operation", RequestFingerprint: "1111111111111111",
		Items: []providerops.ItemInput{{Key: "purchase", TargetType: "purchase", TargetID: purchase.ID}}}
	operation, replayed, err := store.BeginTrafficReset(ctx, input, purchase.ID, now.Add(time.Hour))
	if err != nil || replayed {
		t.Fatalf("BeginTrafficReset() = (%+v, %t, %v)", operation, replayed, err)
	}
	replay, replayed, err := store.BeginTrafficReset(ctx, input, purchase.ID, now.Add(2*time.Hour))
	if err != nil || !replayed || replay.Receipt.ID != operation.Receipt.ID {
		t.Fatalf("BeginTrafficReset(replay) = (%+v, %t, %v)", replay, replayed, err)
	}
	conflict := input
	conflict.RequestFingerprint = "2222222222222222"
	if _, _, err := store.BeginTrafficReset(ctx, conflict, purchase.ID, now.Add(2*time.Hour)); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting replay error = %v, want ErrConflict", err)
	}
	if _, err := store.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, now.Add(3*time.Hour)); err != nil {
		t.Fatalf("BeginProviderOperationAttempt(): %v", err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, "purchase", now.Add(3*time.Hour)); err != nil {
		t.Fatalf("BeginProviderOperationItemAttempt(): %v", err)
	}
	if err := store.CompensateTrafficReset(ctx, operation.Receipt.ID, "RESET_REJECTED", now.Add(3*time.Hour)); err != nil {
		t.Fatalf("CompensateTrafficReset(): %v", err)
	}
	if err := store.CompensateTrafficReset(ctx, operation.Receipt.ID, "RESET_REJECTED", now.Add(4*time.Hour)); err != nil {
		t.Fatalf("CompensateTrafficReset(replay): %v", err)
	}
	assertEventKind(t, store, "manual-reset-refunded:"+operation.Receipt.ID, jobpayload.UserEventManualResetRefunded)
	balance, err := store.Balance(ctx, user.ID)
	if err != nil || balance.Minor != "4699" {
		t.Fatalf("Balance() = (%+v, %v), want 4699", balance, err)
	}
	entries, err := store.ListLedger(ctx, user.ID, 20)
	if err != nil || countLedgerKind(entries, "traffic_reset_compensation") != 1 {
		t.Fatalf("compensation entries = %d, %v", countLedgerKind(entries, "traffic_reset_compensation"), err)
	}
}

func TestMemberRefundCreditsNetAndShiftsQueuedTimeline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 28002)
	combo := saveTestCombo(t, store, "refund-combo", 1_000, 30)
	if _, err := store.AdjustBalance(ctx, user.ID, 5_000, "refund-seed", "test credit", time.Now()); err != nil {
		t.Fatalf("AdjustBalance(): %v", err)
	}
	now := time.Date(2026, 8, 18, 2, 0, 0, 0, time.UTC)
	first := createMemberOperationPurchase(t, store, user.ID, combo.ID, "refund-first", now)
	second := createMemberOperationPurchase(t, store, user.ID, combo.ID, "refund-second", now.Add(time.Minute))
	third := createMemberOperationPurchase(t, store, user.ID, combo.ID, "refund-third", now.Add(2*time.Minute))
	if err := store.MarkPurchaseSyncResult(ctx, first.ID, true, now); err != nil {
		t.Fatalf("MarkPurchaseSyncResult(): %v", err)
	}
	refundAt := now.Add(2 * time.Hour)
	input := providerops.CreateInput{ActorUserID: user.ID, OwnerUserID: user.ID, Kind: purchaseops.OperationRefundKind,
		IdempotencyKey: "member-refund", RequestFingerprint: "3333333333333333",
		Items: []providerops.ItemInput{{Key: "purchase", TargetType: "purchase", TargetID: first.ID}}}
	operation, _, err := store.BeginMemberRefund(ctx, input, first.ID, refundAt)
	if err != nil {
		t.Fatalf("BeginMemberRefund(): %v", err)
	}
	if _, err := store.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, refundAt); err != nil {
		t.Fatalf("BeginProviderOperationAttempt(): %v", err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, "purchase", refundAt); err != nil {
		t.Fatalf("BeginProviderOperationItemAttempt(): %v", err)
	}
	result, err := store.FinalizeMemberRefund(ctx, operation.Receipt.ID, first.ID, refundAt)
	if err != nil || result.Successor == nil || result.Successor.ID != second.ID {
		t.Fatalf("FinalizeMemberRefund() = (%+v, %v)", result, err)
	}
	third, err = store.PurchaseByID(ctx, third.ID)
	if err != nil || !third.ValidFrom.Equal(result.Successor.ValidUntil) {
		t.Fatalf("third queued start = %s, %v; want %s", third.ValidFrom, err, result.Successor.ValidUntil)
	}
	var retiredJobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=? AND payload=? AND status IN ('pending','processing')`,
		jobpayload.ContinuityKind, `{"purchaseId":"`+third.ID+`"}`).Scan(&retiredJobs); err != nil || retiredJobs != 0 {
		t.Fatal("purchase edit regenerated retired continuity work", retiredJobs, err)
	}
	var activatedContinuity int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=? AND payload=?`,
		jobpayload.ContinuityKind, `{"purchaseId":"`+second.ID+`"}`).Scan(&activatedContinuity); err != nil || activatedContinuity != 0 {
		t.Fatalf("activated successor continuity jobs = %d, %v", activatedContinuity, err)
	}
	balance, err := store.Balance(ctx, user.ID)
	if err != nil || balance.Minor != "3000" {
		t.Fatalf("Balance() = (%+v, %v), want 3000", balance, err)
	}
}

func createMemberOperationPurchase(t *testing.T, store *Store, userID, comboID, key string, now time.Time) model.Purchase {
	t.Helper()
	purchase, err := store.CreatePurchase(context.Background(), PurchaseInput{
		UserID: userID, ComboID: comboID, IdempotencyKey: key,
	}, now)
	if err != nil {
		t.Fatalf("CreatePurchase(%s): %v", key, err)
	}
	return purchase
}
