package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type starsMemoryRepository struct {
	cursor       database.TelegramStarCursor
	failAdvance  bool
	refundErr    error
	resolveErr   error
	reviewErr    error
	refunds      []string
	resolved     []string
	reviews      []database.TelegramStarRefundReview
	advancements int
}

func (*starsMemoryRepository) EnsureTelegramStarReconciliation(context.Context, int64, time.Time) error {
	return nil
}
func (r *starsMemoryRepository) TelegramStarReconciliationCursor(context.Context, int64) (database.TelegramStarCursor, error) {
	return r.cursor, nil
}
func (r *starsMemoryRepository) AdvanceTelegramStarReconciliation(_ context.Context, expected database.TelegramStarCursor, offset int64, anchor string, next, _ time.Time) error {
	if r.failAdvance {
		r.failAdvance = false
		return errors.New("cursor commit failed")
	}
	if expected.Generation == r.cursor.Generation {
		r.cursor.Generation++
		r.cursor.NextOffset, r.cursor.Anchor, r.cursor.AvailableAt = offset, anchor, next
		r.advancements++
	}
	return nil
}
func (r *starsMemoryRepository) RecordTelegramStarRefundReview(_ context.Context, proof database.TelegramStarRefundReview, _ time.Time) error {
	if r.reviewErr == nil {
		r.reviews = append(r.reviews, proof)
	}
	return r.reviewErr
}
func (r *starsMemoryRepository) RefundPayment(_ context.Context, _ *string, orderID, _ string, _ time.Time) (model.PaymentOrder, error) {
	if r.refundErr == nil {
		r.refunds = append(r.refunds, orderID)
	}
	return model.PaymentOrder{ID: orderID, Status: "refunded"}, r.refundErr
}
func (r *starsMemoryRepository) ResolvePaymentRefundOperation(_ context.Context, orderID, charge string, _ time.Time) error {
	if r.resolveErr == nil {
		r.resolved = append(r.resolved, orderID+":"+charge)
	}
	return r.resolveErr
}

type starsMemoryBilling struct {
	settled       map[string]bool
	calls         int
	failOnceOrder string
	order         model.PaymentOrder
	validateErr   error
}

func (b *starsMemoryBilling) Settle(_ context.Context, event billing.ProviderEvent) (model.PaymentOrder, bool, error) {
	b.calls++
	if event.OrderID == b.failOnceOrder {
		b.failOnceOrder = ""
		return model.PaymentOrder{}, false, errors.New("settlement transaction failed")
	}
	changed := !b.settled[event.OrderID]
	b.settled[event.OrderID] = true
	return model.PaymentOrder{ID: event.OrderID, Status: "paid"}, changed, nil
}
func (b *starsMemoryBilling) ValidateEvent(context.Context, billing.ProviderEvent) (model.PaymentOrder, error) {
	return b.order, b.validateErr
}

func starsPageJob(t *testing.T, cursor database.TelegramStarCursor) model.OutboxJob {
	t.Helper()
	payload, err := json.Marshal(jobpayload.StarsReconciliationPage{BotID: cursor.BotID, Generation: cursor.Generation})
	if err != nil {
		t.Fatal(err)
	}
	return model.OutboxJob{Kind: jobpayload.StarsReconciliationKind, Payload: string(payload)}
}

func starsIncomingHistory(size int) []telegram.StarTransaction {
	result := make([]telegram.StarTransaction, size)
	for i := range result {
		result[i] = telegram.StarTransaction{ID: fmt.Sprintf("charge-%d", i), Amount: 25, Date: int64(i + 1),
			Source: &telegram.TransactionPartnerUser{Type: "user", TransactionType: "invoice_payment", User: telegram.User{ID: 42}, InvoicePayload: fmt.Sprintf("order-%d", i)}}
	}
	return result
}
