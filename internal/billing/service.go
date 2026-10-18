package billing

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
)

// ErrProviderDisabled means the administrator has not enabled a payment method.
var ErrProviderDisabled = errors.New("payment provider is disabled")

// ErrInvalidOrder means user-controlled checkout selection failed validation.
var ErrInvalidOrder = errors.New("invalid payment order")

// Settings is the trusted runtime configuration surface.
type Settings interface {
	Plaintext(context.Context, string) (string, error)
	Optional(context.Context, string) (string, error)
}

type paymentProfileReader interface {
	PaymentProfiles(context.Context) ([]model.PaymentProfile, error)
}

type paymentProfileChannelReader interface {
	PaymentProfileChannels(string) []model.PaymentChannel
}

type paymentProfileRuntimeReader interface {
	PaymentProfile(context.Context, string, string) (model.PaymentProfileRuntime, error)
}

type paymentProfileByIDRuntimeReader interface {
	PaymentProfileByID(context.Context, string, string) (model.PaymentProfileRuntime, error)
}

// Repository is the transactional billing persistence contract.
type Repository interface {
	CreatePaymentOrder(context.Context, model.PaymentOrder) (model.PaymentOrder, error)
	UpdatePaymentCheckout(context.Context, string, *string, *string, *string, string, string, time.Time) (model.PaymentOrder, error)
	FailPaymentOrder(context.Context, string) error
	PaymentOrderByID(context.Context, string) (model.PaymentOrder, error)
	PaymentOrderForUser(context.Context, string, string) (model.PaymentOrder, error)
	SettlePayment(context.Context, string, string, string, string, string, string, time.Time) (model.PaymentOrder, bool, error)
	UserByID(context.Context, string) (model.User, error)
}

type checkoutDetailsRepository interface {
	UpdatePaymentCheckoutDetails(context.Context, string, *string, *string, *string, *string, *string, *string, string, string, time.Time) (model.PaymentOrder, error)
}

type cancellationRepository interface {
	CancelPaymentOrder(context.Context, string, string, string, time.Time) (model.PaymentOrder, bool, error)
	SetPaymentProviderCancellation(context.Context, string, string, time.Time) error
}

// Service calculates prices and commits verified provider events.
type Service struct {
	repository Repository
	settings   Settings
	gateway    Gateway
	publicURL  *url.URL
	now        func() time.Time
	builtin    BuiltinUSDT
}

// NewService creates a payment service.
func NewService(repository Repository, settings Settings, gateway Gateway, publicURL *url.URL) *Service {
	return &Service{repository: repository, settings: settings, gateway: gateway, publicURL: publicURL, now: time.Now}
}

// CreateOrder computes authoritative pricing, persists the attempt, then requests checkout data.
func (s *Service) CreateOrder(ctx context.Context, user model.User, methodID string, txbMinor int64) (model.PaymentOrder, error) {
	order, err := s.prepareOrder(ctx, user, methodID, txbMinor)
	if err != nil {
		return model.PaymentOrder{}, err
	}
	order, err = s.repository.CreatePaymentOrder(ctx, order)
	if err != nil {
		return model.PaymentOrder{}, err
	}
	return s.createCheckout(ctx, user, order)
}

func (s *Service) prepareOrder(ctx context.Context, user model.User, methodID string, txbMinor int64) (model.PaymentOrder, error) {
	provider, profileID, rail, err := ParseMethodSelection(methodID)
	if err != nil {
		return model.PaymentOrder{}, err
	}
	if provider == "coupon" {
		return model.PaymentOrder{}, fmt.Errorf("%w: coupon funding is not a provider order", ErrInvalidOrder)
	}
	if err := s.validateAddTXBAmount(ctx, txbMinor); err != nil {
		return model.PaymentOrder{}, err
	}
	providerEnabled, settingsErr := s.providerMethodEnabled(ctx, provider, profileID, rail)
	if settingsErr != nil {
		return model.PaymentOrder{}, settingsErr
	}
	if !providerEnabled {
		return model.PaymentOrder{}, ErrProviderDisabled
	}
	rate, err := s.loadNewRate(ctx, provider)
	if err != nil {
		return model.PaymentOrder{}, ErrProviderDisabled
	}
	precision := 2
	if provider == "builtin_usdt" {
		precision = 3
	}
	if provider == "stars" {
		precision = 0
	}
	payable, err := PayableFromTXBPerCurrency(txbMinor, rate, precision)
	if err != nil {
		return model.PaymentOrder{}, err
	}
	now := s.now().UTC()
	expiry := 30 * time.Minute
	if provider == "builtin_usdt" {
		expiry, err = s.builtin.Expiry(ctx)
		if err != nil {
			return model.PaymentOrder{}, err
		}
	}
	canonicalMethodID := provider
	if rail != "" {
		if profileID != "" {
			canonicalMethodID += ":" + profileID
		}
		canonicalMethodID += ":" + rail
	}
	currency := strings.ToUpper(currencyCode(provider))
	if provider == "builtin_usdt" {
		spec, ok := usdt.Network(rail)
		if !ok {
			return model.PaymentOrder{}, ErrInvalidOrder
		}
		currency = spec.Currency()
	}
	return model.PaymentOrder{
		UserID: user.ID, Provider: provider, MethodID: canonicalMethodID, ProviderRail: rail, Status: "creating", TXBMinor: txbMinor,
		PayableAmount: payable, PayableCurrency: currency, RateSnapshot: rate.Canonical(),
		RateDirection: "txb_per_currency", ExpiresAt: now.Add(expiry),
	}, nil
}
