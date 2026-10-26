package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func TestEarlyActivationPreservesDurationsWithoutCredit(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		duration  time.Duration
		automatic bool
	}{{"standard", 7 * 24 * time.Hour, false}, {"custom", 90 * time.Minute, false}, {"automatic", 7 * 24 * time.Hour, true}} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx, store := context.Background(), newTestStore(t)
			now := time.Date(2030, 10, 8, 0, 0, 0, 0, time.UTC)
			core := saveTestCombo(t, store, "current", 1000, 30)
			next := saveTestCombo(t, store, "queued", 100, 7)
			user, current := createAdminWorkflowPurchase(t, store, 49101, core, now)
			queued, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: next.ID, IdempotencyKey: "next"}, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=? WHERE id=?`, stamp(queued.ValidFrom.Add(test.duration)), queued.ID); err != nil {
				t.Fatal(err)
			}
			queued, err = store.PurchaseByID(ctx, queued.ID)
			if err != nil {
				t.Fatal(err)
			}
			later, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: next.ID, IdempotencyKey: "later"}, now)
			if err != nil {
				t.Fatal(err)
			}
			if test.automatic {
				if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET auto_renew_enabled=1 WHERE id=?`, current.ID); err != nil {
					t.Fatal(err)
				}
				if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET auto_renew_source_purchase_id=?,auto_renew_enabled=1 WHERE id=?`, current.ID, queued.ID); err != nil {
					t.Fatal(err)
				}
			}
			balance := adminWorkflowBalance(t, store, user.ID)
			wrong := memberOperationInput(user.ID, later.ID, purchaseops.OperationEarlyActivation, "wrong-order")
			if _, err := store.BeginEarlyActivation(ctx, wrong, current.ID, later.ID, now); !errors.Is(err, purchaseops.ErrIneligible) {
				t.Fatalf("non-earliest error=%v", err)
			}
			input := memberOperationInput(user.ID, queued.ID, purchaseops.OperationEarlyActivation, "activate")
			operation, err := store.BeginEarlyActivation(ctx, input, current.ID, queued.ID, now)
			if err != nil {
				t.Fatal(err)
			}
			after, err := store.PurchaseByID(ctx, queued.ID)
			if err != nil || after.Status != "activating" || !after.ValidFrom.Equal(now) || after.ValidUntil.Sub(after.ValidFrom) != test.duration || after.PriceTXBMinor != queued.PriceTXBMinor {
				t.Fatalf("successor=%+v error=%v", after, err)
			}
			shifted, err := store.PurchaseByID(ctx, later.ID)
			if err != nil || !shifted.ValidFrom.Equal(after.ValidUntil) || shifted.ValidUntil.Sub(shifted.ValidFrom) != later.ValidUntil.Sub(later.ValidFrom) {
				t.Fatalf("later=%+v error=%v", shifted, err)
			}
			forfeited, err := store.PurchaseByID(ctx, current.ID)
			if err != nil || forfeited.Status != "cancelled" || forfeited.AutoRenewEnabled || !forfeited.ValidUntil.Equal(now) {
				t.Fatalf("forfeited=%+v error=%v", forfeited, err)
			}
			if adminWorkflowBalance(t, store, user.ID) != balance {
				t.Fatal("early activation changed wallet")
			}
			replay, err := store.BeginEarlyActivation(ctx, input, current.ID, queued.ID, now)
			if err != nil || replay.Receipt.ID != operation.Receipt.ID {
				t.Fatalf("replay=%v error=%v", replay, err)
			}
			if test.automatic && !after.AutoRenewEnabled {
				t.Fatal("successor renewal preference lost")
			}
		})
	}
}

func TestEarlyActivationConfirmationAndForeignOwnership(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2030, 10, 8, 0, 0, 0, 0, time.UTC)
	core := saveTestCombo(t, store, "confirmation", 1000, 30)
	user, current := createAdminWorkflowPurchase(t, store, 49102, core, now)
	queued, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: core.ID, IdempotencyKey: "queued"}, now)
	if err != nil {
		t.Fatal(err)
	}
	service := purchaseops.NewService(store, nil)
	if _, err := service.ActivateEarly(ctx, user.ID, current.ID, queued.ID, "ACTIVATE", "unconfirmed"); !errors.Is(err, purchaseops.ErrConfirmationRequired) {
		t.Fatalf("confirmation error=%v", err)
	}
	foreign := createTestUser(t, store, 49103)
	input := memberOperationInput(foreign.ID, queued.ID, purchaseops.OperationEarlyActivation, "foreign")
	if _, err := store.BeginEarlyActivation(ctx, input, current.ID, queued.ID, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ownership error=%v", err)
	}
	after, err := store.PurchaseByID(ctx, current.ID)
	if err != nil || after.Status != "active" {
		t.Fatalf("unconfirmed term changed=%+v, %v", after, err)
	}
}

type controlSyncFunc func(context.Context, model.OutboxJob) error

func (f controlSyncFunc) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	return f(ctx, job)
}

func TestEarlyActivationWorkerResumesExistingResetPhases(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Now().UTC()
	core := saveTestCombo(t, store, "retry", 1000, 30)
	user, current := createAdminWorkflowPurchase(t, store, 49104, core, now)
	queued, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: core.ID, IdempotencyKey: "queued"}, now)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := store.BeginEarlyActivation(ctx, memberOperationInput(user.ID, queued.ID, purchaseops.OperationEarlyActivation, "retry-activate"), current.ID, queued.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	calls, invalidations := 0, 0
	worker := purchaseops.NewComboControlWorker(store, controlSyncFunc(func(ctx context.Context, job model.OutboxJob) error {
		if job.Kind != "xnode_apply_entitlement" {
			t.Fatalf("kind=%s", job.Kind)
		}
		calls++
		if calls == 1 {
			if err := store.AdvancePurchaseTrafficReset(ctx, queued.ID, "pending", "quiesced", now); err != nil {
				t.Fatal(err)
			}
			return errors.New("constructed provider outage")
		}
		phase, err := store.PurchaseTrafficResetPhase(ctx, queued.ID)
		if err != nil || phase != "quiesced" {
			t.Fatalf("phase=%s error=%v", phase, err)
		}
		if err := store.AdvancePurchaseTrafficReset(ctx, queued.ID, "quiesced", "reset", now); err != nil {
			return err
		}
		return store.MarkPurchaseSyncResult(ctx, queued.ID, true, now)
	}), func(string) { invalidations++ })
	if err := worker.HandleProviderOperation(ctx, operation, model.OutboxJob{Attempts: 1}); err == nil {
		t.Fatal("outage was ignored")
	}
	operation, err = store.ProviderOperationByID(ctx, operation.Receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.HandleProviderOperation(ctx, operation, model.OutboxJob{Attempts: 2}); err != nil {
		t.Fatal(err)
	}
	completed, err := store.ProviderOperationByID(ctx, operation.Receipt.ID)
	if err != nil || completed.Receipt.Status != "succeeded" || calls != 2 || invalidations != 1 {
		t.Fatalf("receipt=%+v calls=%d invalidations=%d error=%v", completed, calls, invalidations, err)
	}
}
