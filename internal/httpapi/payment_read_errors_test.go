package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/go-chi/chi/v5"
)

type paymentReadRepository struct {
	billing.Repository
	err   error
	calls int
}

func (r *paymentReadRepository) PaymentOrderByID(context.Context, string) (model.PaymentOrder, error) {
	r.calls++
	return model.PaymentOrder{}, r.err
}

func (r *paymentReadRepository) PaymentOrderForUser(context.Context, string, string) (model.PaymentOrder, error) {
	r.calls++
	return model.PaymentOrder{}, r.err
}

func TestPaymentReadHandlersPreserveTransientFailure(t *testing.T) {
	key := make([]byte, 32)
	expires := strconv.FormatInt(time.Now().Add(billing.PaymentReturnCapabilityTTL).Unix(), 10)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("vansail.payment.return.v2\x00ezpay\x00order\x00" + expires))
	capability := "v2." + expires + "." + hex.EncodeToString(mac.Sum(nil))
	for _, test := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"missing", database.ErrNotFound, http.StatusNotFound, "PAYMENT_NOT_FOUND"},
		{"wrapped missing", fmt.Errorf("load: %w", database.ErrNotFound), http.StatusNotFound, "PAYMENT_NOT_FOUND"},
		{"storage failure", errors.New("private database failure"), http.StatusInternalServerError, "PAYMENT_LOAD_FAILED"},
		{"deadline", context.DeadlineExceeded, http.StatusInternalServerError, "PAYMENT_LOAD_FAILED"},
	} {
		for _, route := range []string{"order", "owner return", "capability return"} {
			t.Run(test.name+"/"+route, func(t *testing.T) {
				repository := &paymentReadRepository{err: test.err}
				server := &Server{deps: Dependencies{
					Billing: billing.NewService(repository, nil, nil, nil, key), RequestSigningKey: key,
				}}
				request := httptest.NewRequest(http.MethodGet, "/?capability="+capability, nil)
				routeContext := chi.NewRouteContext()
				routeContext.URLParams.Add("id", "order")
				routeContext.URLParams.Add("orderID", "order")
				routeContext.URLParams.Add("provider", "ezpay")
				ctx := context.WithValue(request.Context(), chi.RouteCtxKey, routeContext)
				ctx = context.WithValue(ctx, userContextKey, model.User{ID: "owner"})
				request = request.WithContext(ctx)
				response := httptest.NewRecorder()
				switch route {
				case "order":
					server.paymentOrder(response, request)
				case "owner return":
					server.paymentReturnOwnerStatus(response, request)
				case "capability return":
					server.paymentReturnStatus(response, request)
				}
				var failure apiError
				if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil {
					t.Fatal(err)
				}
				if response.Code != test.status || failure.Code != test.code || repository.calls != 1 {
					t.Fatalf("status=%d code=%s reads=%d; want %d/%s/1", response.Code, failure.Code, repository.calls, test.status, test.code)
				}
				if strings.Contains(response.Body.String(), "private database failure") {
					t.Fatal("response exposed storage diagnostics")
				}
			})
		}
	}
}
