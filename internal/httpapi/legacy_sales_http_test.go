package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
)

func TestMemberLegacyCheckoutClosesWithoutCreatingPurchase(t *testing.T) {
	s, _, user := testBrowserHTTP(t)
	s.deps.Catalog = catalog.NewService(s.deps.Store, nil, 0)
	user.OnboardingState = "complete"
	before, err := s.deps.Store.Balance(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, handler := range []http.HandlerFunc{s.purchase, s.purchaseQuote} {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/purchases", strings.NewReader(`{"comboId":"retired","couponGrantId":"unused"}`))
		r.Header.Set("Idempotency-Key", "new-fixed-term-attempt")
		r = r.WithContext(context.WithValue(r.Context(), userContextKey, user))
		w := httptest.NewRecorder()
		handler(w, r)
		if w.Code != http.StatusGone || !strings.Contains(w.Body.String(), "LEGACY_PURCHASE_CLOSED") {
			t.Fatalf("new checkout was not closed: %d %s", w.Code, w.Body.String())
		}
	}
	list, err := s.deps.Store.ListPurchases(context.Background(), user.ID)
	if err != nil || len(list) != 0 {
		t.Fatal("closed storefront created a legacy purchase", err)
	}
	after, err := s.deps.Store.Balance(context.Background(), user.ID)
	if err != nil || after != before {
		t.Fatal("closed storefront changed the wallet", err)
	}
}
