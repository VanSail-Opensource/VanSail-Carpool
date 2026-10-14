package billing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"strings"
)

func (s *Service) ValidateEvent(ctx context.Context, event ProviderEvent) (model.PaymentOrder, error) {
	// Built-in receipts enter only the atomic verified-transfer repository path.
	if event.Provider == "builtin_usdt" {
		return model.PaymentOrder{}, ErrInvalidOrder
	}
	order, err := s.repository.PaymentOrderByID(ctx, event.OrderID)
	if err != nil {
		return model.PaymentOrder{}, err
	}
	if order.Provider != event.Provider {
		return model.PaymentOrder{}, database.ErrConflict
	}
	if order.MethodID != "" {
		methodProvider, profileID, _, parseErr := ParseMethodSelection(order.MethodID)
		if parseErr != nil || methodProvider != order.Provider || profileID != event.ProfileID {
			return model.PaymentOrder{}, database.ErrConflict
		}
	}
	if event.Rail != "" && order.ProviderRail != "" && !strings.EqualFold(order.ProviderRail, event.Rail) {
		return model.PaymentOrder{}, database.ErrConflict
	}
	if event.Provider == "bepusdt" {
		var receivingAddress *string
		if order.ActualCryptoAmount != nil || order.ActualCryptoCurrency != nil {
			eventCurrency := event.PayableCurrency
			if eventCurrency == "" && order.ActualCryptoCurrency != nil {
				eventCurrency = *order.ActualCryptoCurrency
			}
			if order.ActualCryptoAmount == nil || order.ActualCryptoCurrency == nil ||
				!strings.EqualFold(*order.ActualCryptoCurrency, eventCurrency) || !Equivalent(*order.ActualCryptoAmount, event.PayableAmount) {
				return model.PaymentOrder{}, database.ErrConflict
			}
			receivingAddress = order.ReceivingAddress
		} else {
			// Orders created before the separate crypto fields were introduced
			// stored the provider's USDT amount/currency in payable_* and the
			// receiving address in qr_payload. Preserve settlement for those
			// immutable pending snapshots; new-direction orders may not use this
			// compatibility path.
			if order.RateDirection != "" && order.RateDirection != "currency_per_txb" {
				return model.PaymentOrder{}, database.ErrConflict
			}
			eventCurrency := event.PayableCurrency
			if eventCurrency == "" {
				eventCurrency = order.PayableCurrency
			}
			if !strings.EqualFold(order.PayableCurrency, eventCurrency) || !Equivalent(order.PayableAmount, event.PayableAmount) {
				return model.PaymentOrder{}, database.ErrConflict
			}
			receivingAddress = order.ReceivingAddress
			if receivingAddress == nil {
				receivingAddress = order.QRPayload
			}
		}
		rate, parseErr := ParseDecimal(order.RateSnapshot)
		var expectedFiat string
		var payableErr error
		if order.RateDirection == "txb_per_currency" {
			expectedFiat, payableErr = PayableFromTXBPerCurrency(order.TXBMinor, rate, 2)
		} else {
			expectedFiat, payableErr = Payable(order.TXBMinor, rate, 2)
		}
		if parseErr != nil || payableErr != nil || !strings.EqualFold(event.FiatCurrency, "USD") || !Equivalent(expectedFiat, event.FiatAmount) {
			return model.PaymentOrder{}, database.ErrConflict
		}
		// Reference versions overload callback token as either the receive
		// address or the literal currency code. Compare it to the exact checkout
		// address whenever the provider supplies address semantics.
		if event.Recipient != "" && !strings.EqualFold(event.Recipient, "USDT") &&
			!strings.EqualFold(event.Recipient, "USDC") &&
			(receivingAddress == nil || event.Recipient != *receivingAddress) {
			return model.PaymentOrder{}, database.ErrConflict
		}
	} else if !strings.EqualFold(order.PayableCurrency, event.PayableCurrency) || !Equivalent(order.PayableAmount, event.PayableAmount) {
		return model.PaymentOrder{}, database.ErrConflict
	}
	if order.ProviderTradeID != nil && event.TradeID != "" && *order.ProviderTradeID != event.TradeID {
		return model.PaymentOrder{}, database.ErrConflict
	}
	if event.TelegramID != nil {
		user, err := s.repository.UserByID(ctx, order.UserID)
		if err != nil || user.TelegramID != *event.TelegramID {
			return model.PaymentOrder{}, database.ErrConflict
		}
	}
	return order, nil
}

// AuthorizeEvent applies the stricter state and expiry checks required before a
// provider is allowed to collect funds. Authoritative paid events remain able to
// settle a just-expired order because the charge has already occurred.
func (s *Service) AuthorizeEvent(ctx context.Context, event ProviderEvent) (model.PaymentOrder, error) {
	order, err := s.ValidateEvent(ctx, event)
	if err != nil {
		return model.PaymentOrder{}, err
	}
	if order.Status != "pending" || !s.now().UTC().Before(order.ExpiresAt) {
		return model.PaymentOrder{}, database.ErrConflict
	}
	return order, nil
}

// Settle commits a previously cryptographically verified provider event exactly once.
func (s *Service) Settle(ctx context.Context, event ProviderEvent) (model.PaymentOrder, bool, error) {
	if event.DedupeKey == "" {
		event.DedupeKey = event.TradeID
	}
	if event.DedupeKey == "" {
		return model.PaymentOrder{}, false, errors.New("provider event has no dedupe key")
	}
	if event.PayloadHash == "" {
		digest := sha256.Sum256([]byte(event.Provider + "\x00" + event.DedupeKey + "\x00" + event.OrderID + "\x00" + event.PayableAmount))
		event.PayloadHash = hex.EncodeToString(digest[:])
	}
	if _, err := s.ValidateEvent(ctx, event); err != nil {
		replays, ok := s.repository.(interface {
			PaymentCallbackReplay(context.Context, string, string, string) (bool, error)
		})
		if !errors.Is(err, database.ErrNotFound) || !ok {
			return model.PaymentOrder{}, false, err
		}
		replayed, replayErr := replays.PaymentCallbackReplay(ctx, event.Provider, event.DedupeKey, event.OrderID)
		if replayErr != nil {
			return model.PaymentOrder{}, false, replayErr
		}
		if !replayed {
			return model.PaymentOrder{}, false, err
		}
		return model.PaymentOrder{ID: event.OrderID, Provider: event.Provider, Status: "paid"}, false, nil
	}
	order, changed, err := s.repository.SettlePayment(ctx, event.Provider, event.DedupeKey, event.PayloadHash,
		event.OrderID, event.TradeID, event.ChargeID, s.now().UTC())
	if err != nil {
		return model.PaymentOrder{}, false, err
	}
	if err := s.resolvePaymentCreateOperation(ctx, event); err != nil {
		return order, changed, err
	}
	return order, changed, nil
}

// OrderForUser returns durable status for payment-sheet polling.
func (s *Service) OrderForUser(ctx context.Context, orderID, userID string) (model.PaymentOrder, error) {
	return s.repository.PaymentOrderForUser(ctx, orderID, userID)
}

// ReturnDetails exposes a narrow receipt projection to the unauthenticated
// provider-return landing page. It deliberately omits the owner, checkout
// URL, QR payload, provider trade IDs, and all provider payload details.
func (s *Service) ReturnDetails(ctx context.Context, provider, orderID string) (model.PaymentReturnDetails, error) {
	order, err := s.repository.PaymentOrderByID(ctx, orderID)
	if err != nil {
		return model.PaymentReturnDetails{}, err
	}
	if !strings.EqualFold(order.Provider, provider) {
		return model.PaymentReturnDetails{}, database.ErrNotFound
	}
	return model.PaymentReturnDetails{
		ID: order.ID, Provider: order.Provider, ProviderRail: order.ProviderRail, Status: order.Status,
		TXB: order.TXB, PayableAmount: order.PayableAmount, PayableCurrency: order.PayableCurrency,
		ActualCryptoAmount: order.ActualCryptoAmount, ActualCryptoCurrency: order.ActualCryptoCurrency,
		CreatedAt: order.CreatedAt, PaidAt: order.PaidAt,
	}, nil
}

// Cancel stops client polling immediately and then performs provider cancellation
// on a best-effort basis. A later verified paid event remains authoritative.
func (s *Service) Cancel(ctx context.Context, orderID, userID string) (model.PaymentOrder, error) {
	repository, ok := s.repository.(cancellationRepository)
	if !ok {
		return model.PaymentOrder{}, errors.New("payment cancellation is unavailable")
	}
	order, changed, err := repository.CancelPaymentOrder(ctx, orderID, userID, "cancelled by user", s.now().UTC())
	if err != nil {
		return model.PaymentOrder{}, err
	}
	if !changed || order.Provider != "bepusdt" || order.ProviderTradeID == nil {
		return order, nil
	}
	canceller, ok := s.gateway.(CancellationGateway)
	if !ok {
		_ = repository.SetPaymentProviderCancellation(ctx, order.ID, "unsupported", s.now().UTC())
		return s.repository.PaymentOrderForUser(ctx, order.ID, userID)
	}
	status := "cancelled"
	if err := canceller.Cancel(ctx, order); err != nil {
		status = "failed"
	}
	_ = repository.SetPaymentProviderCancellation(ctx, order.ID, status, s.now().UTC())
	return s.repository.PaymentOrderForUser(ctx, order.ID, userID)
}
