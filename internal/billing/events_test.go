package billing

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"testing"
	"time"
)

func TestServiceValidateEvent(t *testing.T) {
	t.Parallel()

	tradeID := "trade-1"
	baseOrder := model.PaymentOrder{ID: "order-1", UserID: "user-1", Provider: "ezpay", ProviderRail: "alipay", PayableAmount: "1.00", PayableCurrency: "CNY", ProviderTradeID: &tradeID}
	telegramID := int64(42)
	tests := []struct {
		name      string
		mutate    func(*billingRepository, *ProviderEvent)
		wantError error
	}{
		{name: "valid", mutate: func(_ *billingRepository, event *ProviderEvent) { event.TelegramID = &telegramID }},
		{name: "order lookup", mutate: func(repository *billingRepository, _ *ProviderEvent) { repository.lookupErr = errors.New("lookup") }},
		{name: "provider mismatch", mutate: func(_ *billingRepository, event *ProviderEvent) { event.Provider = "stars" }, wantError: database.ErrConflict},
		{name: "signed subtype mismatch", mutate: func(_ *billingRepository, event *ProviderEvent) { event.Rail = "wxpay" }, wantError: database.ErrConflict},
		{name: "currency mismatch", mutate: func(_ *billingRepository, event *ProviderEvent) { event.PayableCurrency = "USD" }, wantError: database.ErrConflict},
		{name: "amount mismatch", mutate: func(_ *billingRepository, event *ProviderEvent) { event.PayableAmount = "1.01" }, wantError: database.ErrConflict},
		{name: "trade mismatch", mutate: func(_ *billingRepository, event *ProviderEvent) { event.TradeID = "other" }, wantError: database.ErrConflict},
		{name: "telegram mismatch", mutate: func(_ *billingRepository, event *ProviderEvent) { wrong := int64(99); event.TelegramID = &wrong }, wantError: database.ErrConflict},
		{name: "user lookup error", mutate: func(repository *billingRepository, event *ProviderEvent) {
			event.TelegramID = &telegramID
			repository.userErr = errors.New("user lookup")
		}, wantError: database.ErrConflict},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := newBillingRepository()
			repository.orders[baseOrder.ID] = baseOrder
			repository.user = model.User{ID: "user-1", TelegramID: telegramID}
			event := ProviderEvent{Provider: "ezpay", Rail: "alipay", OrderID: baseOrder.ID, TradeID: tradeID, PayableAmount: "1.0", PayableCurrency: "cny"}
			if test.mutate != nil {
				test.mutate(repository, &event)
			}
			_, err := newBillingServiceForTest(repository, &billingSettings{}, &billingGateway{}).ValidateEvent(context.Background(), event)
			if test.wantError != nil && !errors.Is(err, test.wantError) {
				t.Fatalf("ValidateEvent() error = %v, want %v", err, test.wantError)
			}
			if test.wantError == nil && test.name != "order lookup" && err != nil {
				t.Fatalf("ValidateEvent(): %v", err)
			}
			if test.name == "order lookup" && err == nil {
				t.Fatal("ValidateEvent() unexpectedly succeeded")
			}
		})
	}
}

func TestServiceValidateEventRejectsWrongPaymentProfile(t *testing.T) {
	t.Parallel()

	repository := newBillingRepository()
	repository.orders["order-1"] = model.PaymentOrder{
		ID: "order-1", UserID: "user-1", Provider: "ezpay", MethodID: "ezpay:main-account:alipay", ProviderRail: "alipay",
		PayableAmount: "1.00", PayableCurrency: "CNY",
	}
	_, err := newBillingServiceForTest(repository, &billingSettings{}, &billingGateway{}).ValidateEvent(context.Background(), ProviderEvent{
		Provider: "ezpay", ProfileID: "backup-account", Rail: "alipay", OrderID: "order-1", PayableAmount: "1.00", PayableCurrency: "CNY",
	})
	if !errors.Is(err, database.ErrConflict) {
		t.Fatalf("ValidateEvent() error = %v, want profile conflict", err)
	}
}

func TestServiceSettleAndOrderForwarding(t *testing.T) {
	t.Parallel()

	repository := newBillingRepository()
	repository.orders["order-1"] = model.PaymentOrder{ID: "order-1", UserID: "user-1", Provider: "stars", PayableAmount: "5", PayableCurrency: "XTR"}
	service := newBillingServiceForTest(repository, &billingSettings{}, &billingGateway{})

	order, applied, err := service.Settle(context.Background(), ProviderEvent{
		Provider: "stars", OrderID: "order-1", TradeID: "trade-1", ChargeID: "charge-1", PayableAmount: "5.0", PayableCurrency: "xtr",
	})
	if err != nil || !applied || order.ID != "order-1" {
		t.Fatalf("Settle() = (%+v, %t, %v)", order, applied, err)
	}
	if repository.settleDedupe != "trade-1" || len(repository.settleHash) != 64 {
		t.Fatalf("settlement dedupe/hash = %q/%q", repository.settleDedupe, repository.settleHash)
	}

	repository.paymentForUser = model.PaymentOrder{ID: "poll-order"}
	poll, err := service.OrderForUser(context.Background(), "poll-order", "user-1")
	if err != nil || poll.ID != "poll-order" || repository.pollUserID != "user-1" {
		t.Fatalf("OrderForUser() = (%+v, %v), user %q", poll, err, repository.pollUserID)
	}

	_, _, err = service.Settle(context.Background(), ProviderEvent{Provider: "stars", OrderID: "order-1", PayableAmount: "5", PayableCurrency: "XTR"})
	if err == nil {
		t.Fatal("Settle() without dedupe unexpectedly succeeded")
	}
}

func TestServiceReturnDetailsUsesNarrowProjection(t *testing.T) {
	t.Parallel()

	paidAt := time.Date(2026, 8, 11, 1, 2, 3, 0, time.UTC)
	tradeID := "provider-trade-1"
	paymentURL := "https://provider.example/checkout"
	repository := newBillingRepository()
	repository.orders["order-1"] = model.PaymentOrder{
		ID: "order-1", UserID: "user-1", Provider: "bepusdt", ProviderRail: "usdt.trc20", Status: "paid",
		TXB: model.Money{Currency: "TXB", Minor: "2000", Display: "20.00 TXB"}, PayableAmount: "20.00", PayableCurrency: "USD",
		ProviderTradeID: &tradeID, PaymentURL: &paymentURL, CreatedAt: paidAt.Add(-time.Minute), PaidAt: &paidAt,
	}

	details, err := newBillingServiceForTest(repository, &billingSettings{}, &billingGateway{}).ReturnDetails(context.Background(), "bepusdt", "order-1")
	if err != nil {
		t.Fatalf("ReturnDetails(): %v", err)
	}
	if details.ID != "order-1" || details.Provider != "bepusdt" || details.ProviderRail != "usdt.trc20" || details.Status != "paid" || details.PaidAt == nil || !details.PaidAt.Equal(paidAt) {
		t.Fatalf("ReturnDetails() = %+v", details)
	}
	if details.TXB.Display != "20.00 TXB" || details.PayableAmount != "20.00" || details.PayableCurrency != "USD" {
		t.Fatalf("ReturnDetails() amount = %+v", details)
	}
}

func TestServiceAuthorizeEventRequiresLivePendingOrder(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		status    string
		expiresAt time.Time
		wantError bool
	}{
		{name: "pending", status: "pending", expiresAt: now.Add(time.Minute)},
		{name: "creating", status: "creating", expiresAt: now.Add(time.Minute), wantError: true},
		{name: "paid", status: "paid", expiresAt: now.Add(time.Minute), wantError: true},
		{name: "refunded", status: "refunded", expiresAt: now.Add(time.Minute), wantError: true},
		{name: "failed", status: "failed", expiresAt: now.Add(time.Minute), wantError: true},
		{name: "expired by status", status: "expired", expiresAt: now.Add(time.Minute), wantError: true},
		{name: "expired by time", status: "pending", expiresAt: now, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := newBillingRepository()
			repository.orders["order-1"] = model.PaymentOrder{ID: "order-1", UserID: "user-1", Provider: "stars", Status: test.status, PayableAmount: "5", PayableCurrency: "XTR", ExpiresAt: test.expiresAt}
			_, err := newBillingServiceForTest(repository, &billingSettings{}, &billingGateway{}).AuthorizeEvent(context.Background(), ProviderEvent{
				Provider: "stars", OrderID: "order-1", PayableAmount: "5", PayableCurrency: "XTR",
			})
			if test.wantError && !errors.Is(err, database.ErrConflict) {
				t.Fatalf("AuthorizeEvent() error = %v, want ErrConflict", err)
			}
			if !test.wantError && err != nil {
				t.Fatalf("AuthorizeEvent(): %v", err)
			}
		})
	}
}

func TestServiceValidateBEPusdtFiatAndCryptoAmounts(t *testing.T) {
	t.Parallel()

	recipient := "TXExactReceiveAddress"
	repository := newBillingRepository()
	repository.orders["order-1"] = model.PaymentOrder{
		ID: "order-1", UserID: "user-1", Provider: "bepusdt", TXBMinor: 1_000,
		PayableAmount: "1.00", PayableCurrency: "USD", RateSnapshot: "0.1", ReceivingAddress: &recipient,
		ActualCryptoAmount: ptrString("0.950001"), ActualCryptoCurrency: ptrString("USDT"),
	}
	service := newBillingServiceForTest(repository, &billingSettings{}, &billingGateway{})
	event := ProviderEvent{
		Provider: "bepusdt", OrderID: "order-1", PayableAmount: "0.950001", PayableCurrency: "USDT",
		FiatAmount: "1.00", FiatCurrency: "USD", Recipient: recipient,
	}
	if _, err := service.ValidateEvent(context.Background(), event); err != nil {
		t.Fatalf("ValidateEvent(valid): %v", err)
	}
	event.FiatAmount = "1.01"
	if _, err := service.ValidateEvent(context.Background(), event); !errors.Is(err, database.ErrConflict) {
		t.Fatalf("ValidateEvent(fiat mismatch) error = %v, want ErrConflict", err)
	}
	event.FiatAmount = "1.00"
	event.PayableAmount = "0.950002"
	if _, err := service.ValidateEvent(context.Background(), event); !errors.Is(err, database.ErrConflict) {
		t.Fatalf("ValidateEvent(crypto mismatch) error = %v, want ErrConflict", err)
	}
	event.PayableAmount = "0.950001"
	event.Recipient = "different-address"
	if _, err := service.ValidateEvent(context.Background(), event); !errors.Is(err, database.ErrConflict) {
		t.Fatalf("ValidateEvent(recipient mismatch) error = %v, want ErrConflict", err)
	}
	event.Recipient = "USDT"
	if _, err := service.ValidateEvent(context.Background(), event); err != nil {
		t.Fatalf("ValidateEvent(overloaded token currency): %v", err)
	}
}

func TestServiceValidateLegacyBEPusdtExpiredSnapshotAfterCleanup(t *testing.T) {
	t.Parallel()

	recipient := "TXLegacyReceiveAddress"
	repository := newBillingRepository()
	repository.orders["legacy-order"] = model.PaymentOrder{
		ID: "legacy-order", UserID: "user-1", Provider: "bepusdt", Status: "expired", TXBMinor: 1_000,
		PayableAmount: "0.950001", PayableCurrency: "USDT", RateSnapshot: "0.1", RateDirection: "currency_per_txb",
		QRPayload: &recipient,
	}
	service := newBillingServiceForTest(repository, &billingSettings{}, &billingGateway{})
	event := ProviderEvent{
		Provider: "bepusdt", OrderID: "legacy-order", PayableAmount: "0.950001", PayableCurrency: "USDT",
		FiatAmount: "1.00", FiatCurrency: "USD", Recipient: recipient,
	}
	if _, err := service.ValidateEvent(context.Background(), event); err != nil {
		t.Fatalf("ValidateEvent(legacy valid): %v", err)
	}
	event.Recipient = "different-address"
	if _, err := service.ValidateEvent(context.Background(), event); !errors.Is(err, database.ErrConflict) {
		t.Fatalf("ValidateEvent(legacy recipient mismatch) error = %v, want ErrConflict", err)
	}
	event.Recipient = recipient
	repository.orders["legacy-order"] = model.PaymentOrder{
		ID: "legacy-order", UserID: "user-1", Provider: "bepusdt", TXBMinor: 1_000,
		PayableAmount: "0.950001", PayableCurrency: "USDT", RateSnapshot: "10", RateDirection: "txb_per_currency",
	}
	if _, err := service.ValidateEvent(context.Background(), event); !errors.Is(err, database.ErrConflict) {
		t.Fatalf("ValidateEvent(new direction without crypto snapshot) error = %v, want ErrConflict", err)
	}
}
