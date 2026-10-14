package billing

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// ProviderCreateRequest is the server-owned checkout input passed to an adapter.
type ProviderCreateRequest struct {
	Provider        string
	ProfileID       string
	MethodID        string
	Rail            string
	OrderID         string
	TelegramID      int64
	TXBMinor        int64
	PayableAmount   string
	PayableCurrency string
	NotifyURL       string
	ReturnURL       string
	RedirectURL     string
}

// ProviderCheckout is the exact display contract returned by a provider.
type ProviderCheckout struct {
	TradeID              *string
	PaymentURL           *string
	QRPayload            *string
	ReceivingAddress     *string
	ActualCryptoAmount   *string
	ActualCryptoCurrency *string
	PayableAmount        string
	PayableCurrency      string
	ExpiresAt            time.Time
}

// ProviderEvent is a verified, normalized authoritative payment event.
type ProviderEvent struct {
	Provider        string
	ProfileID       string
	Rail            string
	OrderID         string
	TradeID         string
	ChargeID        string
	PayableAmount   string
	PayableCurrency string
	FiatAmount      string
	FiatCurrency    string
	Recipient       string
	DedupeKey       string
	PayloadHash     string
	TelegramID      *int64
}

// Gateway creates provider-specific checkouts. It receives only server-owned values.
type Gateway interface {
	Create(context.Context, ProviderCreateRequest) (ProviderCheckout, error)
}

// CancellationGateway is implemented by providers that expose a signed order
// cancellation operation. Absence is a valid best-effort outcome.
type CancellationGateway interface {
	Cancel(context.Context, model.PaymentOrder) error
}
