package billing

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestAuthoritativeEventOwnerLookupCanRetryAfterStorageFailure(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"paid", "refunded"} {
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			repository := newBillingRepository()
			repository.orders["order"] = model.PaymentOrder{ID: "order", UserID: "member", Provider: "stars", Status: status, PayableAmount: "5", PayableCurrency: "XTR"}
			repository.user = model.User{ID: "member", TelegramID: 42}
			service := newBillingServiceForTest(repository, &billingSettings{}, &billingGateway{})
			event := ProviderEvent{Provider: "stars", OrderID: "order", PayableAmount: "5", PayableCurrency: "XTR", TelegramID: &repository.user.TelegramID}
			for _, failure := range []error{errors.New("owner storage unavailable"), context.DeadlineExceeded, context.Canceled} {
				repository.userErr = failure
				if _, err := service.ValidateEvent(context.Background(), event); !errors.Is(err, failure) || errors.Is(err, database.ErrConflict) {
					t.Fatal("temporary owner lookup was converted to a payment mismatch", err)
				}
			}
			repository.userErr = nil
			order, err := service.ValidateEvent(context.Background(), event)
			if err != nil || order.ID != "order" || order.Status != status {
				t.Fatal("same authoritative callback failed after owner lookup recovered", order, err)
			}
		})
	}
}
