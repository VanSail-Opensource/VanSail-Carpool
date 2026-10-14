package app

import (
	"context"
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (a paymentAdapter) ProviderRefunded(ctx context.Context, order model.PaymentOrder) (bool, error) {
	if order.Provider != "stars" {
		return false, nil
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
			partner := transaction.Receiver
			if partner != nil && partner.Type == "user" && partner.TransactionType == "invoice_payment" &&
				partner.InvoicePayload == order.ID && partner.User.ID == user.TelegramID {
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
