package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func TestMemberControlsRebaseFarFutureQueuedTermsExactly(t *testing.T) {
	for _, kind := range []string{purchaseops.OperationRefundKind, purchaseops.OperationEarlyActivation} {
		t.Run(kind, func(t *testing.T) {
			ctx, store := context.Background(), newTestStore(t)
			now := time.Date(2026, 11, 12, 1, 2, 3, 123456789, time.UTC)
			owner := createTestUser(t, store, 90410)
			combo := saveTestCombo(t, store, "Far future queue", 100, 30)
			if _, err := store.AdjustBalance(ctx, owner.ID, 500, "queue-rebase-seed", "fixture", now); err != nil {
				t.Fatal(err)
			}
			first := createMemberOperationPurchase(t, store, owner.ID, combo.ID, "queue-first", now)
			second := createMemberOperationPurchase(t, store, owner.ID, combo.ID, "queue-second", now.Add(time.Minute))
			third := createMemberOperationPurchase(t, store, owner.ID, combo.ID, "queue-third", now.Add(2*time.Minute))
			if err := store.MarkPurchaseSyncResult(ctx, first.ID, true, now); err != nil {
				t.Fatal(err)
			}
			future := time.Date(2500, 1, 1, 0, 0, 0, 987654321, time.UTC)
			const term = 30 * 24 * time.Hour
			for index, id := range []string{second.ID, third.ID} {
				from := future.Add(time.Duration(index) * term)
				if _, err := store.DB().ExecContext(ctx, "UPDATE purchases SET valid_from=$1,valid_until=$2 WHERE id=$3", stamp(from), stamp(from.Add(term)), id); err != nil {
					t.Fatal(err)
				}
			}
			at := now.Add(2 * time.Hour)
			wantBalance := int64(200)
			if kind == purchaseops.OperationRefundKind {
				input := memberOperationInput(owner.ID, first.ID, kind, "far-future-refund")
				operation, _, err := store.BeginMemberRefund(ctx, input, first.ID, at)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, at); err != nil {
					t.Fatal(err)
				}
				if _, err := store.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, "purchase", at); err != nil {
					t.Fatal(err)
				}
				if _, err := store.FinalizeMemberRefund(ctx, operation.Receipt.ID, first.ID, at); err != nil {
					t.Fatal(err)
				}
				wantBalance += 100
			} else {
				input := memberOperationInput(owner.ID, second.ID, kind, "far-future-activation")
				if _, err := store.BeginEarlyActivation(ctx, input, first.ID, second.ID, at); err != nil {
					t.Fatal(err)
				}
			}
			for index, id := range []string{second.ID, third.ID} {
				purchase, err := store.PurchaseByID(ctx, id)
				from := at.Add(time.Duration(index) * term)
				if err != nil || !purchase.ValidFrom.Equal(from) || !purchase.ValidUntil.Equal(from.Add(term)) || purchase.PriceTXBMinor != 100 {
					t.Fatalf("term %d changed its exact offset, duration or price: %+v, %v", index, purchase, err)
				}
				wantStatus := "queued"
				if index == 0 {
					wantStatus = "activating"
				}
				if purchase.Status != wantStatus {
					t.Fatal("unexpected successor state", purchase.Status, wantStatus)
				}
			}
			if got := adminWorkflowBalance(t, store, owner.ID); got != wantBalance {
				t.Fatal("rebasing changed the wrong wallet amount", got, wantBalance)
			}
		})
	}
}

func TestPurchaseTimestampRebaseRejectsUnrepresentableResults(t *testing.T) {
	first := time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	target := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	at := first.AddDate(2, 0, 0)
	if _, err := rebasePurchaseTimestamp(at, first, target); !errors.Is(err, ErrConflict) {
		t.Fatal("out-of-range timestamp was accepted", err)
	}
}
