package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func TestAdminEditCannotReopenRefundedFinancialHistory(t *testing.T) {
	t.Parallel()
	for _, lane := range []string{"admin_refund", "admin_cancel", "queued_cancel", "member_refund"} {
		t.Run(lane, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			now := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
			actor := createTestAdmin(t, store, 91801)
			combo := saveTestCombo(t, store, "immutable-refund", 1_700, 30)
			user, purchase := createAdminWorkflowPurchase(t, store, 91802, combo, now)
			if lane == "queued_cancel" {
				var err error
				purchase, err = store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID,
					IdempotencyKey: "refundable-queued"}, now.Add(time.Second))
				if err != nil {
					t.Fatal(err)
				}
			}
			edit := AdminEntitlementEditInput{ActorUserID: actor.ID, UserID: user.ID, PurchaseID: purchase.ID,
				IdempotencyKey: "accepted-edit", RequestFingerprint: "accepted-edit-fingerprint", Reason: "correct term",
				ExpectedUpdatedAt: purchase.UpdatedAt, ComboID: purchase.ComboID, Status: purchase.Status,
				ValidFrom: purchase.ValidFrom, ValidUntil: purchase.ValidUntil.Add(time.Hour),
				TrafficLimitBytes: purchase.TrafficLimitBytes, ResetStrategy: purchase.ResetStrategy, SquadUUIDs: purchase.SquadUUIDs}
			updated, err := store.EditAdminEntitlement(ctx, edit, now.Add(time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			beforeBalance := adminWorkflowBalance(t, store, user.ID)
			credited := refundHistoryPurchase(t, store, actor.ID, updated, lane, now.Add(2*time.Minute))
			refunded, err := store.PurchaseByID(ctx, purchase.ID)
			if err != nil || refunded.Status != "cancelled" {
				t.Fatalf("refunded purchase = %+v, %v", refunded, err)
			}
			var operationsBefore int
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM provider_operations`).Scan(&operationsBefore); err != nil {
				t.Fatal(err)
			}
			reopen := edit
			reopen.IdempotencyKey, reopen.RequestFingerprint = "reopen-refund", "reopen-refund-fingerprint"
			reopen.ExpectedUpdatedAt, reopen.Status = refunded.UpdatedAt, "active"
			if _, err := store.EditAdminEntitlement(ctx, reopen, now.Add(3*time.Minute)); !errors.Is(err, ErrConflict) {
				t.Fatalf("reopening settled history = %v; want conflict", err)
			}
			var operationsAfter int
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM provider_operations`).Scan(&operationsAfter); err != nil || operationsAfter != operationsBefore {
				t.Fatalf("rejected edit left command receipt: %d -> %d, %v", operationsBefore, operationsAfter, err)
			}
			// A previously accepted command still replays after a later refund.
			if replay, err := store.EditAdminEntitlement(ctx, edit, now.Add(4*time.Minute)); err != nil || replay.Status != "cancelled" {
				t.Fatalf("accepted edit replay = %+v, %v", replay, err)
			}
			// The other cancellation lane cannot credit a second refund after the
			// blocked edit. Partial refunds also keep their settled original row.
			if _, err := store.CancelAdminPurchase(ctx, actor.ID, purchase.ID, "repeat cancellation", now.Add(5*time.Minute)); err != nil {
				t.Fatal(err)
			}
			if balance := adminWorkflowBalance(t, store, user.ID); balance != beforeBalance+credited {
				t.Fatalf("refund moved wallet twice: %d; want %d", balance, beforeBalance+credited)
			}
		})
	}
}

func refundHistoryPurchase(t *testing.T, store *Store, actorID string, purchase model.Purchase, lane string, now time.Time) int64 {
	t.Helper()
	ctx := context.Background()
	switch lane {
	case "admin_refund":
		_, err := store.RefundAdminEntitlement(ctx, AdminEntitlementRefundInput{ActorUserID: actorID, UserID: purchase.UserID,
			PurchaseID: purchase.ID, IdempotencyKey: "partial-refund", RequestFingerprint: "partial-refund-fingerprint",
			Reason: "partial settlement", AmountTXBMinor: 500}, now)
		if err != nil {
			t.Fatal(err)
		}
		return 500
	case "admin_cancel":
		if _, err := store.CancelAdminPurchase(ctx, actorID, purchase.ID, "cancel entitlement", now); err != nil {
			t.Fatal(err)
		}
	case "queued_cancel":
		if _, err := store.CancelQueuedPurchase(ctx, purchase.UserID, purchase.ID, "cancel queued term", now); err != nil {
			t.Fatal(err)
		}
	case "member_refund":
		input := providerops.CreateInput{ActorUserID: purchase.UserID, OwnerUserID: purchase.UserID,
			Kind: purchaseops.OperationRefundKind, IdempotencyKey: "refund-member", RequestFingerprint: "member-refund-fingerprint",
			Items: []providerops.ItemInput{{Key: "purchase", TargetType: "purchase", TargetID: purchase.ID}}}
		operation, _, err := store.BeginMemberRefund(ctx, input, purchase.ID, now)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, now); err != nil {
			t.Fatal(err)
		}
		if _, err := store.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, "purchase", now); err != nil {
			t.Fatal(err)
		}
		if _, err := store.FinalizeMemberRefund(ctx, operation.Receipt.ID, purchase.ID, now); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown refund lane %q", lane)
	}
	return purchase.PriceTXBMinor
}
