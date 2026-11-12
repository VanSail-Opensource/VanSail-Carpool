package app

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type starRefundUsers struct{ paymentProfileTestStore }

func (starRefundUsers) UserByID(context.Context, string) (model.User, error) {
	return model.User{ID: "member", TelegramID: 42}, nil
}

func TestProviderRefundedRequiresExactAuthoritativeCharge(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*telegram.StarTransaction, *model.PaymentOrder)
		want   bool
	}{
		{name: "exact charge", want: true},
		{name: "positive outgoing amount", mutate: func(tx *telegram.StarTransaction, _ *model.PaymentOrder) { tx.Amount = 25 }, want: true},
		{name: "another charge for same invoice", mutate: func(tx *telegram.StarTransaction, _ *model.PaymentOrder) { tx.ID = "other-charge" }},
		{name: "wrong Stars amount", mutate: func(tx *telegram.StarTransaction, _ *model.PaymentOrder) { tx.Amount = -24 }},
		{name: "fractional amount", mutate: func(tx *telegram.StarTransaction, _ *model.PaymentOrder) { tx.NanostarAmount = 1 }},
		{name: "absolute amount overflow", mutate: func(tx *telegram.StarTransaction, _ *model.PaymentOrder) { tx.Amount = math.MinInt64 }},
		{name: "incoming payment", mutate: func(tx *telegram.StarTransaction, _ *model.PaymentOrder) { tx.Source, tx.Receiver = tx.Receiver, nil }},
		{name: "both directions", mutate: func(tx *telegram.StarTransaction, _ *model.PaymentOrder) { tx.Source = tx.Receiver }},
		{name: "another owner", mutate: func(tx *telegram.StarTransaction, _ *model.PaymentOrder) { tx.Receiver.User.ID = 99 }},
		{name: "another invoice", mutate: func(tx *telegram.StarTransaction, _ *model.PaymentOrder) { tx.Receiver.InvoicePayload = "other-order" }},
		{name: "another currency", mutate: func(_ *telegram.StarTransaction, order *model.PaymentOrder) { order.PayableCurrency = "USD" }},
		{name: "unrelated outgoing transaction", mutate: func(tx *telegram.StarTransaction, _ *model.PaymentOrder) {
			tx.Receiver.TransactionType = "paid_media_payment"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			charge := "charge"
			order := model.PaymentOrder{ID: "order", UserID: "member", Provider: "stars", ProviderTradeID: &charge,
				PayableAmount: "25", PayableCurrency: "XTR", Status: "paid"}
			transaction := starRefundTransaction()
			if test.mutate != nil {
				test.mutate(&transaction, &order)
			}
			provider := starsHistoryTelegramForTest(t, func(offset, limit int) ([]telegram.StarTransaction, error) {
				if offset != 0 || limit != 100 {
					t.Errorf("history page = %d, %d", offset, limit)
				}
				return []telegram.StarTransaction{transaction}, nil
			})
			got, err := (paymentAdapter{telegram: provider, users: &starRefundUsers{}}).ProviderRefunded(context.Background(), order)
			if err != nil || got != test.want {
				t.Fatalf("ProviderRefunded = %t, %v; want %t", got, err, test.want)
			}
		})
	}
}

func TestProviderRefundedSearchesLaterPageAndPropagatesFailure(t *testing.T) {
	charge := "charge"
	order := model.PaymentOrder{ID: "order", UserID: "member", Provider: "stars", ProviderTradeID: &charge,
		PayableAmount: "25", PayableCurrency: "XTR", Status: "paid"}
	for _, fail := range []bool{false, true} {
		provider := starsHistoryTelegramForTest(t, func(offset, _ int) ([]telegram.StarTransaction, error) {
			if offset == 0 {
				return make([]telegram.StarTransaction, 100), nil
			}
			if offset != 100 {
				t.Errorf("unexpected history offset %d", offset)
			}
			if fail {
				return nil, errors.New("provider unavailable")
			}
			return []telegram.StarTransaction{starRefundTransaction()}, nil
		})
		got, err := (paymentAdapter{telegram: provider, users: &starRefundUsers{}}).ProviderRefunded(context.Background(), order)
		if fail && (err == nil || got) || !fail && (err != nil || !got) {
			t.Fatalf("ProviderRefunded with fail=%t: %t, %v", fail, got, err)
		}
	}
	order.ProviderTradeID = nil
	if got, err := (paymentAdapter{}).ProviderRefunded(context.Background(), order); err == nil || got {
		t.Fatalf("missing retained charge = %t, %v", got, err)
	}
}

func starRefundTransaction() telegram.StarTransaction {
	return telegram.StarTransaction{ID: "charge", Amount: -25, Receiver: &telegram.TransactionPartnerUser{
		Type: "user", TransactionType: "invoice_payment", InvoicePayload: "order", User: telegram.User{ID: 42},
	}}
}
