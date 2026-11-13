package app

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/testdatabase"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestStarsRefundBeyondFastPathResolvesDurableReview(t *testing.T) {
	ctx, now := context.Background(), time.Now().UTC()
	db, err := testdatabase.Open(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	store := database.NewStore(db)
	user, _, err := store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 42, FirstName: "Stars owner"}, true)
	if err != nil {
		t.Fatal(err)
	}
	order := model.PaymentOrder{ID: "stars-history-order", UserID: user.ID, Provider: "stars", MethodID: "stars", Status: "creating",
		TXBMinor: 2500, PayableAmount: "25", PayableCurrency: "XTR", RateSnapshot: "1", RateDirection: "txb_per_currency", ExpiresAt: now.Add(time.Hour)}
	if _, _, err := store.BeginPaymentOrderOperation(ctx, order, providerops.CreateInput{
		ActorUserID: user.ID, OwnerUserID: user.ID, Kind: billing.OperationCreateKind, IdempotencyKey: "history-create",
		RequestFingerprint: strings.Repeat("a", 64), Items: []providerops.ItemInput{{Key: "payment", TargetType: "payment_order", TargetID: order.ID}},
	}, now); err != nil {
		t.Fatal(err)
	}
	service := billing.NewService(store, nil, nil, nil, nil)
	partner := &telegram.TransactionPartnerUser{Type: "user", TransactionType: "invoice_payment", User: telegram.User{ID: 42}, InvoicePayload: order.ID}
	incoming := telegram.StarTransaction{ID: "retained-charge", Amount: 25, Date: 1, Source: partner}
	refund := telegram.StarTransaction{ID: incoming.ID, Amount: 25, Date: 1101, Receiver: partner}
	event, outgoing, ok := normalizeStarTransaction(incoming)
	if !ok || outgoing || starTransactionHash(incoming) == starTransactionHash(refund) {
		t.Fatal("incoming/refund proof identities were collapsed")
	}
	if _, _, err := service.Settle(ctx, event); err != nil {
		t.Fatal(err)
	}
	operation, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{
		ActorUserID: user.ID, OwnerUserID: user.ID, Kind: providerops.KindPaymentRefund, IdempotencyKey: "history-refund",
		RequestFingerprint: strings.Repeat("b", 64), SealedTarget: `{"reason":"ambiguous refund"}`,
		Items: []providerops.ItemInput{{Key: "payment", TargetType: "payment_order", TargetID: order.ID}},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, "payment", now); err != nil {
		t.Fatal(err)
	}
	review := providerops.Completion{Status: providerops.StatusPendingReview, ErrorCode: "REFUND_OUTCOME_AMBIGUOUS"}
	if _, err := store.CompleteProviderOperationItem(ctx, operation.Receipt.ID, "payment", review, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteProviderOperation(ctx, operation.Receipt.ID, review, now); err != nil {
		t.Fatal(err)
	}
	history := make([]telegram.StarTransaction, 1101)
	for i := range history {
		history[i] = telegram.StarTransaction{ID: fmt.Sprintf("unrelated-%d", i), Amount: 1, Date: int64(i + 1), Source: &telegram.TransactionPartnerUser{Type: "fragment"}}
	}
	history[0], history[len(history)-1] = incoming, refund
	var lastOffset atomic.Int64
	provider := starsHistoryTelegramForTest(t, func(offset, limit int) ([]telegram.StarTransaction, error) {
		lastOffset.Store(int64(offset))
		return history[min(offset, len(history)):min(offset+limit, len(history))], nil
	})
	paid, err := store.PaymentOrderByID(ctx, order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed, err := (paymentAdapter{telegram: provider, users: store}).ProviderRefunded(ctx, paid); err != nil || confirmed {
		t.Fatalf("bounded fast path accepted unavailable proof = %t, %v", confirmed, err)
	}
	if err := store.EnsureTelegramStarReconciliation(ctx, 123, now); err != nil {
		t.Fatal(err)
	}
	for pages := 0; ; pages++ {
		if pages >= 20 {
			t.Fatal("Stars history never reached refund beyond1,000 records")
		}
		cursor, err := store.TelegramStarReconciliationCursor(ctx, 123)
		if err != nil {
			t.Fatal(err)
		}
		if cursor.NextOffset == int64(len(history)) {
			break
		}
		worker := &starsReconciler{botID: 123, provider: provider, repository: store, billing: service, now: time.Now}
		if err := worker.HandleOutbox(ctx, starsPageJob(t, cursor)); err != nil {
			t.Fatal(err)
		}
	}
	resolved, err := store.ProviderOperationByID(ctx, operation.Receipt.ID)
	if err != nil || resolved.Receipt.Status != string(providerops.StatusSucceeded) || resolved.ProviderReference != incoming.ID {
		t.Fatalf("ambiguous refund receipt = %+v, %v", resolved, err)
	}
	refunded, err := store.PaymentOrderByID(ctx, order.ID)
	if err != nil || refunded.Status != "refunded" {
		t.Fatalf("refund order = %+v, %v", refunded, err)
	}
	if balance, err := store.Balance(ctx, user.ID); err != nil || balance.Minor != "0" {
		t.Fatalf("replayed original payment or missing reversal = %+v, %v", balance, err)
	}
	cursor, err := store.TelegramStarReconciliationCursor(ctx, 123)
	if err != nil {
		t.Fatal(err)
	}
	worker := &starsReconciler{botID: 123, provider: provider, repository: store, billing: service, now: time.Now}
	if err := worker.HandleOutbox(ctx, starsPageJob(t, cursor)); err != nil || lastOffset.Load() != 1100 {
		t.Fatalf("caught-up history restarted full scan: offset=%d error=%v", lastOffset.Load(), err)
	}
	refunds, err := store.ListRefunds(ctx, 100)
	if err != nil || len(refunds) != 1 {
		t.Fatalf("refund was not exactly once: %+v, %v", refunds, err)
	}
}
