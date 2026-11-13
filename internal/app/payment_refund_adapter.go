package app

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (a paymentAdapter) ProviderRefunded(ctx context.Context, order model.PaymentOrder) (bool, error) {
	if order.Provider != "stars" {
		return false, nil
	}
	if order.ProviderTradeID == nil || *order.ProviderTradeID == "" {
		return false, errors.New("stars payment has no Telegram charge id")
	}
	user, err := a.users.UserByID(ctx, order.UserID)
	if err != nil {
		return false, err
	}
	for offset := 0; offset < 1_000; offset += 100 {
		transactions, err := a.telegram.GetStarTransactions(ctx, offset, 100)
		if err != nil {
			return false, err
		}
		for _, transaction := range transactions {
			event, refund, ok := normalizeStarTransaction(transaction)
			if ok && refund && event.OrderID == order.ID && event.TradeID == *order.ProviderTradeID &&
				event.TelegramID != nil && *event.TelegramID == user.TelegramID &&
				strings.EqualFold(order.PayableCurrency, event.PayableCurrency) && billing.Equivalent(order.PayableAmount, event.PayableAmount) {
				return true, nil
			}
		}
		if len(transactions) < 100 {
			break
		}
	}
	return false, nil
}

func (a paymentAdapter) DefinitiveRefundFailure(err error) bool {
	var apiError *telegram.APIError
	return errors.As(err, &apiError) && apiError.HTTPStatus >= http.StatusBadRequest &&
		apiError.HTTPStatus < http.StatusInternalServerError && apiError.HTTPStatus != http.StatusRequestTimeout &&
		apiError.HTTPStatus != http.StatusTooManyRequests
}
