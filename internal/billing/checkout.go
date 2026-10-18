package billing

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Service) createCheckout(ctx context.Context, user model.User, order model.PaymentOrder) (model.PaymentOrder, error) {
	request, err := s.providerCreateRequest(ctx, user, order)
	if err != nil {
		_ = s.repository.FailPaymentOrder(ctx, order.ID)
		return model.PaymentOrder{}, err
	}
	checkout, err := s.gateway.Create(ctx, request)
	if err != nil {
		_ = s.repository.FailPaymentOrder(ctx, order.ID)
		return model.PaymentOrder{}, fmt.Errorf("create provider checkout: %w", err)
	}
	result, err := s.storeCheckout(ctx, order, checkout)
	if err != nil {
		if errors.Is(err, ErrInvalidOrder) {
			_ = s.repository.FailPaymentOrder(ctx, order.ID)
		}
		// The provider has accepted the create request, so the outcome must
		// remain reconcilable when only the local checkout write failed.
		return model.PaymentOrder{}, err
	}
	return result, nil
}

func (s *Service) providerCreateRequest(ctx context.Context, user model.User, order model.PaymentOrder) (ProviderCreateRequest, error) {
	provider, profileID, rail, err := ParseMethodSelection(order.MethodID)
	if err != nil || provider != order.Provider || rail != order.ProviderRail {
		return ProviderCreateRequest{}, fmt.Errorf("create provider checkout: %w: invalid stored method", ErrInvalidOrder)
	}
	request := ProviderCreateRequest{
		Provider: provider, ProfileID: profileID, MethodID: order.MethodID, Rail: rail, OrderID: order.ID,
		TelegramID: user.TelegramID, TXBMinor: order.TXBMinor, PayableAmount: order.PayableAmount,
		PayableCurrency: order.PayableCurrency, NotifyURL: s.absolute("/api/v1/webhooks/" + provider),
		ReturnURL:   s.absolute("/api/v1/payments/return/" + provider + "/" + url.PathEscape(order.ID)),
		RedirectURL: s.absolute("/api/v1/payments/return/" + provider + "/" + url.PathEscape(order.ID)),
	}
	if provider == "bepusdt" {
		secret, secretErr := s.providerCredential(ctx, provider, profileID)
		if secretErr != nil {
			return ProviderCreateRequest{}, fmt.Errorf("create callback capability: %w", secretErr)
		}
		request.NotifyURL = s.absolute("/api/v1/webhooks/bepusdt/" + callbackCapability(secret, order.ID))
	}
	return request, nil
}

func (s *Service) storeCheckout(ctx context.Context, order model.PaymentOrder, checkout ProviderCheckout) (model.PaymentOrder, error) {
	if checkout.PayableAmount == "" {
		checkout.PayableAmount = order.PayableAmount
	}
	if checkout.PayableCurrency == "" {
		checkout.PayableCurrency = order.PayableCurrency
	}
	if !strings.EqualFold(checkout.PayableCurrency, order.PayableCurrency) ||
		!Equivalent(checkout.PayableAmount, order.PayableAmount) {
		return model.PaymentOrder{}, fmt.Errorf("create provider checkout: %w: provider pricing conflicts with immutable order", ErrInvalidOrder)
	}
	checkout.PayableAmount, checkout.PayableCurrency = order.PayableAmount, order.PayableCurrency
	if err := validateCryptoCheckout(order.Provider, checkout); err != nil {
		return model.PaymentOrder{}, err
	}
	if order.Provider == "builtin_usdt" && checkout.ActualCryptoCurrency != nil && !strings.EqualFold(*checkout.ActualCryptoCurrency, order.PayableCurrency) {
		return model.PaymentOrder{}, fmt.Errorf("create provider checkout: %w: token conflicts with immutable order", ErrInvalidOrder)
	}
	if checkout.ExpiresAt.IsZero() {
		checkout.ExpiresAt = order.ExpiresAt
	}
	if repository, ok := s.repository.(checkoutDetailsRepository); ok {
		return repository.UpdatePaymentCheckoutDetails(ctx, order.ID, checkout.TradeID, checkout.PaymentURL,
			checkout.QRPayload, checkout.ReceivingAddress, checkout.ActualCryptoAmount, checkout.ActualCryptoCurrency,
			checkout.PayableAmount, checkout.PayableCurrency, checkout.ExpiresAt)
	}
	return s.repository.UpdatePaymentCheckout(ctx, order.ID, checkout.TradeID, checkout.PaymentURL,
		checkout.QRPayload, checkout.PayableAmount, checkout.PayableCurrency, checkout.ExpiresAt)
}

func validateCryptoCheckout(provider string, checkout ProviderCheckout) error {
	if (checkout.ActualCryptoAmount == nil) != (checkout.ActualCryptoCurrency == nil) {
		return fmt.Errorf("create provider checkout: %w: incomplete crypto pricing", ErrInvalidOrder)
	}
	if checkout.ActualCryptoAmount == nil {
		return nil
	}
	actual, err := ParseDecimal(*checkout.ActualCryptoAmount)
	currency := strings.ToUpper(*checkout.ActualCryptoCurrency)
	if (provider != "bepusdt" && provider != "builtin_usdt") || err != nil || !actual.Positive() || (currency != "USDT" && currency != "USDC") || (provider == "builtin_usdt" && (checkout.ReceivingAddress == nil || *checkout.ReceivingAddress == "")) {
		return fmt.Errorf("create provider checkout: %w: invalid crypto pricing", ErrInvalidOrder)
	}
	return nil
}
