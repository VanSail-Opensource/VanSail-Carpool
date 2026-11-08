package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// PaymentReturnCapabilityTTL starts at immutable order creation, never at a return visit.
const PaymentReturnCapabilityTTL = 30 * time.Minute

func paymentReturnProvider(provider string) bool { return provider == "ezpay" || provider == "bepusdt" }

func returnCapabilityMAC(key []byte, provider, orderID, expires string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("vansail.payment.return.v2\x00" + provider + "\x00" + orderID + "\x00" + expires))
	return mac.Sum(nil)
}

// Only checkout creation issues capabilities. Retries for one order keep its expiry.
func (s *Service) checkoutReturnURL(order model.PaymentOrder) (string, error) {
	if len(s.returnKey) < 32 || order.ID == "" || order.UserID == "" || order.CreatedAt.IsZero() || !paymentReturnProvider(order.Provider) {
		return "", ErrInvalidOrder
	}
	expires := strconv.FormatInt(order.CreatedAt.Add(PaymentReturnCapabilityTTL).Unix(), 10)
	capability := "v2." + expires + "." + hex.EncodeToString(returnCapabilityMAC(s.returnKey, order.Provider, order.ID, expires))
	target, err := url.Parse(s.absolute("/api/v1/payments/return/" + order.Provider + "/" + url.PathEscape(order.ID)))
	if err != nil {
		return "", err
	}
	query := target.Query()
	query.Set("capability", capability)
	target.RawQuery = query.Encode()
	return target.String(), nil
}

// ValidPaymentReturnCapability accepts only the domain-separated checkout format.
// Old capabilities issued publicly by the navigation route cannot authenticate.
func ValidPaymentReturnCapability(key []byte, provider, orderID, capability string, now time.Time) bool {
	if len(key) < 32 || !paymentReturnProvider(provider) || orderID == "" || len(orderID) > 128 ||
		strings.ContainsAny(orderID, "\x00\r\n") || len(capability) > 128 {
		return false
	}
	parts := strings.Split(capability, ".")
	if len(parts) != 3 || parts[0] != "v2" || len(parts[2]) != sha256.Size*2 {
		return false
	}
	expires, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || expires <= now.Unix() || strconv.FormatInt(expires, 10) != parts[1] {
		return false
	}
	signature, err := hex.DecodeString(parts[2])
	return err == nil && hmac.Equal(signature, returnCapabilityMAC(key, provider, orderID, parts[1]))
}

func paymentReturnDetails(order model.PaymentOrder) model.PaymentReturnDetails {
	return model.PaymentReturnDetails{
		ID: order.ID, Provider: order.Provider, ProviderRail: order.ProviderRail, Status: order.Status,
		TXB: order.TXB, PayableAmount: order.PayableAmount, PayableCurrency: order.PayableCurrency,
		ActualCryptoAmount: order.ActualCryptoAmount, ActualCryptoCurrency: order.ActualCryptoCurrency,
		CreatedAt: order.CreatedAt, PaidAt: order.PaidAt,
	}
}

// ReturnDetailsForOwner supports legacy returns through authenticated ownership only.
func (s *Service) ReturnDetailsForOwner(ctx context.Context, provider, orderID, owner string) (model.PaymentReturnDetails, error) {
	order, err := s.repository.PaymentOrderForUser(ctx, orderID, owner)
	if err != nil {
		return model.PaymentReturnDetails{}, err
	}
	if !paymentReturnProvider(provider) || order.Provider != provider {
		return model.PaymentReturnDetails{}, database.ErrNotFound
	}
	return paymentReturnDetails(order), nil
}
