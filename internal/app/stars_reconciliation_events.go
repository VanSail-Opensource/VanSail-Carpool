package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (w *starsReconciler) applyTransaction(ctx context.Context, transaction telegram.StarTransaction) error {
	event, refund, ok := normalizeStarTransaction(transaction)
	if !ok {
		return nil
	}
	if !refund {
		_, _, err := w.billing.Settle(ctx, event)
		if errors.Is(err, database.ErrConflict) || errors.Is(err, database.ErrNotFound) {
			return nil
		}
		return err
	}
	order, err := w.billing.ValidateEvent(ctx, event)
	if err != nil && !errors.Is(err, database.ErrConflict) && !errors.Is(err, database.ErrNotFound) {
		return err
	}
	if err != nil || order.ProviderTradeID == nil || *order.ProviderTradeID != event.TradeID || (order.Status != "paid" && order.Status != "refunded") {
		reason := "PAYMENT_REFUND_PROOF_MISMATCH"
		if errors.Is(err, database.ErrNotFound) {
			reason = "PAYMENT_REFUND_ORDER_NOT_FOUND"
		}
		return w.repository.RecordTelegramStarRefundReview(ctx, database.TelegramStarRefundReview{
			BotID: w.botID, OrderID: event.OrderID, ChargeID: event.TradeID, TelegramID: *event.TelegramID,
			PayableAmount: event.PayableAmount, Currency: event.PayableCurrency, ProofHash: starTransactionHash(transaction), Reason: reason,
		}, w.now().UTC())
	}
	if _, err := w.repository.RefundPayment(ctx, nil, event.OrderID, "Telegram Stars reconciliation refund", w.now().UTC()); err != nil {
		return err
	}
	return w.repository.ResolvePaymentRefundOperation(ctx, event.OrderID, transaction.ID, w.now().UTC())
}

func starTransactionHash(transaction telegram.StarTransaction) string {
	// Telegram may return the user's current profile on historical records.
	// Only transaction identity/proof fields belong in the durable boundary.
	type partnerProof struct {
		Type            string `json:"type"`
		TransactionType string `json:"transactionType"`
		UserID          int64  `json:"userId"`
		InvoicePayload  string `json:"invoicePayload"`
	}
	project := func(partner *telegram.TransactionPartnerUser) *partnerProof {
		if partner == nil {
			return nil
		}
		return &partnerProof{partner.Type, partner.TransactionType, partner.User.ID, partner.InvoicePayload}
	}
	proof := struct {
		ID             string        `json:"id"`
		Amount         int64         `json:"amount"`
		NanostarAmount int64         `json:"nanostarAmount"`
		Date           int64         `json:"date"`
		Source         *partnerProof `json:"source,omitempty"`
		Receiver       *partnerProof `json:"receiver,omitempty"`
	}{transaction.ID, transaction.Amount, transaction.NanostarAmount, transaction.Date, project(transaction.Source), project(transaction.Receiver)}
	encoded, _ := json.Marshal(proof)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func normalizeStarTransaction(transaction telegram.StarTransaction) (billing.ProviderEvent, bool, bool) {
	if transaction.NanostarAmount != 0 || transaction.ID == "" || (transaction.Source != nil && transaction.Receiver != nil) {
		return billing.ProviderEvent{}, false, false
	}
	amount := transaction.Amount
	if amount < 0 {
		amount = -amount
	}
	if amount <= 0 {
		return billing.ProviderEvent{}, false, false
	}
	partner := transaction.Source
	refund := false
	if partner == nil {
		partner = transaction.Receiver
		refund = partner != nil
	}
	if partner == nil || partner.Type != "user" || partner.TransactionType != "invoice_payment" || partner.InvoicePayload == "" || partner.User.ID <= 0 {
		return billing.ProviderEvent{}, false, false
	}
	telegramID := partner.User.ID
	event := billing.ProviderEvent{
		Provider: "stars", OrderID: partner.InvoicePayload, TradeID: transaction.ID,
		PayableAmount: strconv.FormatInt(amount, 10), PayableCurrency: "XTR", TelegramID: &telegramID,
	}
	if !refund {
		event.DedupeKey = transaction.ID
	}
	return event, refund, true
}
