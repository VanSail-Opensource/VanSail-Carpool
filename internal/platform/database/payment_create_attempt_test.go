package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func paymentAttemptFixture(t *testing.T, provider string) (*Store, model.PaymentOrder, providerops.Operation, time.Time) {
	t.Helper()
	ctx := context.Background()
	store := newTestStore(t)
	owner := createTestUser(t, store, 554301)
	now := time.Now().UTC()
	order, err := store.CreatePaymentOrder(ctx, model.PaymentOrder{UserID: owner.ID, Provider: provider,
		Status: "creating", TXBMinor: 100, PayableAmount: "1", PayableCurrency: "USD", RateSnapshot: "1", ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	operation, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: owner.ID, OwnerUserID: owner.ID,
		Kind: "payment_create", IdempotencyKey: "payment-attempt", RequestFingerprint: "0123456789abcdef",
		Items: []providerops.ItemInput{{Key: "payment", TargetType: "payment_order", TargetID: order.ID}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	operation, err = store.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	return store, order, operation, now
}

func TestPaymentCreateAttemptRechecksCurrentOrderAndOwner(t *testing.T) {
	for _, scenario := range []string{"live", "canceled", "expired", "purging", "foreign-owner", "parent-terminal", "item-terminal", "external-processing", "builtin-processing"} {
		t.Run(scenario, func(t *testing.T) {
			provider := "bepusdt"
			if scenario == "builtin-processing" {
				provider = "builtin_usdt"
			}
			store, order, operation, now := paymentAttemptFixture(t, provider)
			ctx := context.Background()
			var changeErr error
			var wantErr error
			switch scenario {
			case "canceled":
				_, _, changeErr = store.CancelPaymentOrder(ctx, order.ID, order.UserID, "stop", now)
			case "expired":
				_, changeErr = store.DB().ExecContext(ctx, `UPDATE payment_orders SET expires_at=$1 WHERE id=$2`, stamp(now), order.ID)
			case "purging":
				_, changeErr = store.DB().ExecContext(ctx, `UPDATE users SET account_status='purging',account_generation=account_generation+1 WHERE id=$1`, order.UserID)
				wantErr = ErrAccountUnavailable
			case "foreign-owner":
				foreign := createTestUser(t, store, 554302)
				_, changeErr = store.DB().ExecContext(ctx, `UPDATE payment_orders SET user_id=$1 WHERE id=$2`, foreign.ID, order.ID)
				wantErr = ErrConflict
			case "parent-terminal":
				_, changeErr = store.CompleteProviderOperation(ctx, operation.Receipt.ID, providerops.Completion{Status: providerops.StatusFailed}, now)
				wantErr = ErrConflict
			case "item-terminal", "external-processing", "builtin-processing":
				_, changeErr = store.BeginProviderOperationItemAttempt(ctx, operation.Receipt.ID, "payment", now)
				if scenario == "item-terminal" && changeErr == nil {
					_, changeErr = store.CompleteProviderOperationItem(ctx, operation.Receipt.ID, "payment", providerops.Completion{Status: providerops.StatusFailed, ErrorCode: "EXISTING_RESULT"}, now)
				}
			}
			if changeErr != nil {
				t.Fatal(changeErr)
			}
			item, current, allowed, err := store.BeginPaymentCreateAttempt(ctx, operation.Receipt.ID, "payment", now)
			wantAllowed := scenario == "live" || scenario == "builtin-processing"
			if !errors.Is(err, wantErr) || allowed != wantAllowed {
				t.Fatal("incorrect external admission", allowed, err, wantErr)
			}
			if err != nil {
				return
			}
			if allowed && (item.Status != providerops.StatusProcessing || item.AttemptStartedAt == nil || current.ID != order.ID) {
				t.Fatal("admission did not retain its exact attempt and order", item, current)
			}
			if scenario == "canceled" || scenario == "expired" {
				if item.Status != providerops.StatusQueued || item.AttemptStartedAt != nil || current.Status == "creating" {
					t.Fatal("stopped order obtained an external attempt", item, current)
				}
			}
			if scenario == "item-terminal" && (item.Status != providerops.StatusFailed || item.ErrorCode != "EXISTING_RESULT") {
				t.Fatal("terminal evidence was replaced", item)
			}
		})
	}
}

func TestPaymentCreateAttemptObservesCancellationCommittedAheadOfItsRowLock(t *testing.T) {
	store, order, operation, now := paymentAttemptFixture(t, "bepusdt")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE payment_orders SET cancelled_at=$1,cancel_reason='stop' WHERE id=$2`, stamp(now), order.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		item, current, allowed, err := store.BeginPaymentCreateAttempt(ctx, operation.Receipt.ID, "payment", now)
		if err == nil && (allowed || current.CancelledAt == nil || item.AttemptStartedAt != nil) {
			err = errors.New("cancellation was not serialized with external attempt admission")
		}
		done <- err
	}()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
