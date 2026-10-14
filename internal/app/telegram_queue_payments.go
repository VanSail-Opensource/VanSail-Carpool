package app

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func (a *queuedTelegram) CreateStarsInvoiceLink(ctx context.Context, request telegram.StarsInvoiceRequest) (string, error) {
	return upstreamqueue.Do(ctx, a.queue, func(callCtx context.Context) (string, error) {
		return a.client.CreateStarsInvoiceLink(callCtx, request)
	})
}
func (a *queuedTelegram) AnswerPreCheckoutQuery(ctx context.Context, queryID string, approved bool, message string) error {
	return upstreamqueue.Execute(ctx, a.queue, func(callCtx context.Context) error {
		return a.client.AnswerPreCheckoutQuery(callCtx, queryID, approved, message)
	})
}
func (a *queuedTelegram) GetStarTransactions(ctx context.Context, offset, limit int) ([]telegram.StarTransaction, error) {
	return upstreamqueue.Do(ctx, a.queue, func(callCtx context.Context) ([]telegram.StarTransaction, error) {
		return a.client.GetStarTransactions(callCtx, offset, limit)
	})
}
func (a *queuedTelegram) RefundStarPayment(ctx context.Context, userID int64, chargeID string) error {
	return upstreamqueue.Execute(ctx, a.queue, func(callCtx context.Context) error { return a.client.RefundStarPayment(callCtx, userID, chargeID) })
}
