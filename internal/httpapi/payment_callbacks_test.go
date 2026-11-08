package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestPaymentReturnAcceptsOnlyDocumentedProviders(t *testing.T) {
	t.Parallel()

	publicURL, err := url.Parse("https://example.test/miniapp")
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{deps: Dependencies{PublicURL: publicURL}}
	router := chi.NewRouter()
	router.Get("/api/v1/payments/return/{provider}", server.paymentReturn)

	for _, provider := range []string{"ezpay", "bepusdt"} {
		t.Run(provider, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodGet, "/api/v1/payments/return/"+provider+"?order=payment-1", nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusSeeOther {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusSeeOther)
			}
			location, err := url.Parse(response.Header().Get("Location"))
			if err != nil {
				t.Fatalf("parse redirect: %v", err)
			}
			if location.Path != "/miniapp/payment-result" || location.Query().Get("provider") != provider || location.Query().Get("paymentOrder") != "payment-1" || location.Query().Get("paymentCapability") != "" {
				t.Fatalf("redirect location = %q", location.String())
			}
		})
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/payments/return/stripe?order=payment-1", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unsupported provider status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if response.Header().Get("Location") != "" {
		t.Fatalf("unsupported provider redirected to %q", response.Header().Get("Location"))
	}
	var body apiError
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Code != "NOT_FOUND" {
		t.Fatalf("error code = %q, want NOT_FOUND", body.Code)
	}
}
