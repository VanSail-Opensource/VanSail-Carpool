package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/browserauth"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/go-chi/chi/v5"
)

type returnCheckoutFixture struct{ request billing.ProviderCreateRequest }

func (*returnCheckoutFixture) Plaintext(_ context.Context, key string) (string, error) {
	return map[string]string{"billing.ezpay.enabled": "true", "billing.ezpay.methods": "alipay", "billing.rate.txb_per_cny": "1"}[key], nil
}
func (f *returnCheckoutFixture) Optional(ctx context.Context, key string) (string, error) {
	return f.Plaintext(ctx, key)
}
func (f *returnCheckoutFixture) Create(_ context.Context, request billing.ProviderCreateRequest) (billing.ProviderCheckout, error) {
	f.request = request
	return billing.ProviderCheckout{}, nil
}

func TestPaymentReturnCheckoutCapabilityAndOwnerFallback(t *testing.T) {
	server, _, owner := testBrowserHTTP(t)
	ctx := context.Background()
	fixture := &returnCheckoutFixture{}
	server.deps.RequestSigningKey = make([]byte, 32)
	server.deps.Billing = billing.NewService(server.deps.Store, fixture, fixture, server.deps.PublicURL, server.deps.RequestSigningKey)
	order, err := server.deps.Billing.CreateOrder(ctx, owner, "ezpay", 100)
	if err != nil {
		t.Fatal(err)
	}
	checkout, err := url.Parse(fixture.request.ReturnURL)
	if err != nil {
		t.Fatal(err)
	}
	capability := checkout.Query().Get("capability")
	if capability == "" {
		t.Fatal("checkout omitted return capability")
	}
	router := chi.NewRouter()
	router.Get("/api/v1/payments/return/{provider}/{orderID}", server.paymentReturn)
	router.Get("/api/v1/payments/return/{provider}/{orderID}/status", server.paymentReturnStatus)
	path := "/api/v1/payments/return/ezpay/" + order.ID
	for _, supplied := range []string{"", "expired.invalid", capability} {
		request := httptest.NewRequest(http.MethodGet, path+"?capability="+url.QueryEscape(supplied), nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		location, parseErr := url.Parse(response.Header().Get("Location"))
		if parseErr != nil || response.Code != http.StatusSeeOther || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("invalid navigation response")
		}
		want := ""
		if supplied == capability {
			want = capability
		}
		if location.Query().Get("paymentCapability") != want {
			t.Fatal("navigation minted or renewed a capability")
		}
	}
	for _, test := range []struct {
		path, cap string
		want      int
	}{
		{path, capability, http.StatusOK},
		{path, "", http.StatusUnauthorized},
		{path, strings.TrimPrefix(capability, "v2."), http.StatusUnauthorized},
		{path, capability + "0", http.StatusUnauthorized},
		{"/api/v1/payments/return/bepusdt/" + order.ID, capability, http.StatusUnauthorized},
		{"/api/v1/payments/return/ezpay/another-order", capability, http.StatusUnauthorized},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path+"/status?capability="+url.QueryEscape(test.cap), nil))
		if response.Code != test.want {
			t.Fatalf("%s: status %d, want %d: %s", test.path, response.Code, test.want, response.Body)
		}
		if test.want == http.StatusOK {
			var projection map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &projection); err != nil {
				t.Fatal(err)
			}
			for _, private := range []string{"userId", "paymentUrl", "qrPayload", "providerTradeId", "rateSnapshot"} {
				if _, ok := projection[private]; ok {
					t.Fatalf("receipt exposed %s", private)
				}
			}
		}
	}
	foreign, _, err := server.deps.Store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: 80306, FirstName: "Other"}, true)
	if err != nil {
		t.Fatal(err)
	}
	server.deps.BrowserAuth = browserauth.New(server.deps.Store, server.deps.PublicURL, []int64{owner.TelegramID, foreign.TelegramID})
	server.adminTelegramIDs[foreign.TelegramID] = struct{}{}
	for _, user := range []model.User{owner, foreign} {
		token, err := ids.Token(32)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte(token))
		now := time.Now().UTC()
		if err := server.deps.Store.CreateBrowserSession(ctx, hash[:], user.ID, now, now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodGet, path+"/status", nil)
		request.Header.Set("X-TXC-Session-Scope", "browser")
		signBrowserTestRequest(t, server, request, token, "")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		want := http.StatusOK
		if user.ID != owner.ID {
			want = http.StatusNotFound
		}
		if response.Code != want {
			t.Fatalf("owner fallback: got %d, want %d: %s", response.Code, want, response.Body)
		}
	}
	after, err := server.deps.Store.PaymentOrderByID(ctx, order.ID)
	if err != nil || after.Status != order.Status || after.PaidAt != nil {
		t.Fatal("return changed settlement state")
	}
}
