package billing

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func assertCheckoutReturnURL(t *testing.T, service *Service, request ProviderCreateRequest, order model.PaymentOrder) {
	t.Helper()
	parsed, err := url.Parse(request.ReturnURL)
	if err != nil {
		t.Fatal(err)
	}
	capability := parsed.Query().Get("capability")
	if paymentReturnProvider(order.Provider) && !ValidPaymentReturnCapability(service.returnKey, order.Provider, order.ID, capability, service.now()) {
		t.Fatal("checkout omitted a valid order-bound capability")
	}
	parsed.RawQuery = ""
	if parsed.String() != "https://example.test/base/api/v1/payments/return/"+order.Provider+"/"+order.ID || request.ReturnURL != request.RedirectURL {
		t.Fatal("checkout return destination changed")
	}
}

func TestCheckoutReturnCapabilityIsBoundedAndImmutable(t *testing.T) {
	t.Parallel()
	repository := newBillingRepository()
	gateway := &billingGateway{}
	service := newBillingServiceForTest(repository, &billingSettings{values: map[string]string{
		"billing.ezpay.enabled": "true", "billing.rate.txb_per_cny": "1",
	}}, gateway)
	order, err := service.CreateOrder(context.Background(), model.User{ID: "user-1"}, "ezpay", 100)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(gateway.request.ReturnURL)
	if err != nil {
		t.Fatal(err)
	}
	capability := parsed.Query().Get("capability")
	if !ValidPaymentReturnCapability(service.returnKey, order.Provider, order.ID, capability, service.now()) {
		t.Fatal("checkout did not issue a usable receipt capability")
	}
	for _, test := range []struct {
		provider, id, capability string
		now                      time.Time
	}{
		{"bepusdt", order.ID, capability, service.now()},
		{order.Provider, "another-order", capability, service.now()},
		{order.Provider, order.ID, capability + "0", service.now()},
		{order.Provider, order.ID, strings.TrimPrefix(capability, "v2."), service.now()},
		{order.Provider, order.ID, capability, order.CreatedAt.Add(PaymentReturnCapabilityTTL)},
	} {
		if ValidPaymentReturnCapability(service.returnKey, test.provider, test.id, test.capability, test.now) {
			t.Fatal("accepted an expired, legacy, altered or foreign capability")
		}
	}
	service.now = func() time.Time { return order.CreatedAt.Add(24 * time.Hour) }
	retried, err := service.checkoutReturnURL(order)
	if err != nil || retried != gateway.request.ReturnURL {
		t.Fatal("retry renewed the original checkout capability")
	}
	service.returnKey = nil
	if _, err := service.checkoutReturnURL(order); err == nil {
		t.Fatal("issued a capability without a signing key")
	}
}

func TestLegacyReturnDetailsRequireOwnerRepositoryLookup(t *testing.T) {
	t.Parallel()
	repository := newBillingRepository()
	repository.paymentForUser = model.PaymentOrder{ID: "order-1", Provider: "ezpay", UserID: "owner", Status: "paid"}
	service := newBillingServiceForTest(repository, nil, nil)
	details, err := service.ReturnDetailsForOwner(context.Background(), "ezpay", "order-1", "owner")
	if err != nil || details.ID != "order-1" || repository.pollUserID != "owner" {
		t.Fatalf("owner lookup failed: %+v, %v", details, err)
	}
	if _, err := service.ReturnDetailsForOwner(context.Background(), "bepusdt", "order-1", "owner"); err == nil {
		t.Fatal("accepted a receipt from another provider")
	}
}
