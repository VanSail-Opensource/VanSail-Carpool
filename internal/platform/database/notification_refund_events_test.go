package database

import (
	"context"
	"testing"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func TestMemberRefundNoticesFollowFinalOutcome(t *testing.T) {
	for _, test := range []struct {
		name   string
		status providerops.Status
		want   string
	}{
		{"failed", providerops.StatusFailed, jobpayload.UserEventMemberRefundFailed},
		{"pending review", providerops.StatusPendingReview, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, store, _, operation, now := refundNoticeFixture(t)
			completion := providerops.Completion{Status: test.status, ErrorCode: purchaseops.ReasonTrafficUsed, ResultJSON: "{}"}
			if _, err := store.CompleteProviderOperationItem(ctx, operation.Receipt.ID, "purchase", completion, now); err != nil {
				t.Fatal(err)
			}
			if _, err := store.CompleteProviderOperation(ctx, operation.Receipt.ID, completion, now); err != nil {
				t.Fatal(err)
			}
			var count int
			if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notification_events WHERE source_id=?`, operation.Receipt.ID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if test.want == "" && count != 0 {
				t.Fatalf("review produced %d outcome events", count)
			}
			if test.want != "" {
				if count != 1 {
					t.Fatalf("failed refund produced %d events", count)
				}
				assertEventKind(t, store, "member-refund-failed:"+operation.Receipt.ID, test.want)
			}
		})
	}
}

func TestMemberRefundSuccessQueuesReceiptOnce(t *testing.T) {
	ctx, store, purchaseID, operation, now := refundNoticeFixture(t)
	if _, err := store.FinalizeMemberRefund(ctx, operation.Receipt.ID, purchaseID, now); err != nil {
		t.Fatal(err)
	}
	assertEventKind(t, store, "member-refund-completed:"+operation.Receipt.ID, jobpayload.UserEventMemberRefundCompleted)
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_notification_events WHERE source_id=?`, operation.Receipt.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("refund receipt count = %d, %v", count, err)
	}
}

func refundNoticeFixture(t *testing.T) (context.Context, *Store, string, providerops.Operation, time.Time) {
	t.Helper()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 90_100)
	combo := saveTestCombo(t, store, "Refund", 100, 30)
	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 500, "refund-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	purchase := createMemberOperationPurchase(t, store, user.ID, combo.ID, "refund-source", now)
	if err := store.MarkPurchaseSyncResult(ctx, purchase.ID, true, now); err != nil {
		t.Fatal(err)
	}
	input := providerops.CreateInput{ActorUserID: user.ID, OwnerUserID: user.ID, Kind: purchaseops.OperationRefundKind,
		IdempotencyKey: "refund-operation", RequestFingerprint: "3333333333333333",
		Items: []providerops.ItemInput{{Key: "purchase", TargetType: "purchase", TargetID: purchase.ID}}}
	operation, _, err := store.BeginMemberRefund(ctx, input, purchase.ID, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, "purchase", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	return ctx, store, purchase.ID, operation, now.Add(time.Hour)
}
