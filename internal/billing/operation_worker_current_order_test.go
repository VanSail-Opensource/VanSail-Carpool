package billing

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type paymentPreflightBoundaryRepository struct {
	*database.Store
	afterOwnerRead func(context.Context) error
}

func (r *paymentPreflightBoundaryRepository) UserByID(ctx context.Context, id string) (model.User, error) {
	user, err := r.Store.UserByID(ctx, id)
	if err == nil && r.afterOwnerRead != nil {
		action := r.afterOwnerRead
		r.afterOwnerRead = nil
		err = action(ctx)
	}
	return user, err
}

func TestPaymentCreateRechecksStateAfterPreflightBeforeProviderCall(t *testing.T) {
	for _, scenario := range []string{"cancellation", "expiry", "recorded-item"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			store, user := newPaymentOperationStore(t, 9889)
			repository := &paymentPreflightBoundaryRepository{Store: store}
			gateway := &bepusdtRetryGateway{}
			service := newBillingServiceForTest(repository, &billingSettings{values: map[string]string{
				"billing.bepusdt.enabled": "true", "billing.rate.txb_per_usd": "1",
			}}, gateway)
			now := billingTestNow()
			service.now = func() time.Time { return now }
			queued, err := service.QueueOrder(ctx, user, "bepusdt", 100, "changed-during-preflight")
			if err != nil {
				t.Fatal(err)
			}
			order, err := store.PaymentOrderByID(ctx, queued.PaymentOrderID)
			if err != nil {
				t.Fatal(err)
			}
			wantCode := ""
			repository.afterOwnerRead = func(ctx context.Context) error {
				switch scenario {
				case "cancellation":
					wantCode = "PAYMENT_CREATE_CANCELLED"
					_, _, err := store.CancelPaymentOrder(ctx, order.ID, user.ID, "member-cancelled", now)
					return err
				case "expiry":
					wantCode, now = "PAYMENT_CREATE_EXPIRED", order.ExpiresAt
				case "recorded-item":
					wantCode = "EXISTING_TERMINAL_RESULT"
					if _, err := store.BeginProviderOperationItemAttempt(ctx, queued.Operation.ID, "payment", now); err != nil {
						return err
					}
					_, err := store.CompleteProviderOperationItem(ctx, queued.Operation.ID, "payment", providerops.Completion{
						Status: providerops.StatusFailed, ErrorCode: wantCode, ResultJSON: `{"retained":true}`,
					}, now)
					return err
				}
				return nil
			}
			current, operation := exercisePaymentOperation(t, service, queued)
			if gateway.calls != 0 || operation.Receipt.Status != string(providerops.StatusFailed) ||
				operation.Receipt.ErrorCode == nil || *operation.Receipt.ErrorCode != wantCode {
				t.Fatalf("stale preflight created a checkout or replaced evidence: calls=%d operation=%+v", gateway.calls, operation)
			}
			if scenario == "cancellation" && current.CancelledAt == nil {
				t.Fatal("cancellation was not preserved")
			}
			if scenario == "expiry" && current.Status != "expired" {
				t.Fatal("elapsed immutable checkout deadline was not persisted", current)
			}
			if scenario == "recorded-item" && operation.ResultJSON != `{"retained":true}` {
				t.Fatal("terminal item result was not replayed exactly", operation.ResultJSON)
			}
			if _, replay := exercisePaymentOperation(t, service, queued); replay.ResultJSON != operation.ResultJSON || gateway.calls != 0 {
				t.Fatal("terminal replay attempted another checkout", replay, gateway.calls)
			}
		})
	}
}
