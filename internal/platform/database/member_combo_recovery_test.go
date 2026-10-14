package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func TestFailedEarlyActivationCanResumeAfterAdminRetryOrArtifactRetention(t *testing.T) {
	for _, mode := range []string{"admin retry", "retention"} {
		t.Run(mode, func(t *testing.T) {
			ctx, store := context.Background(), newTestStore(t)
			now := time.Now().UTC()
			combo := saveTestCombo(t, store, "recover-control", 1000, 30)
			user, current := createAdminWorkflowPurchase(t, store, 31940, combo, now)
			queued, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "recover-queued"}, now)
			if err != nil {
				t.Fatal(err)
			}
			operation, err := store.BeginEarlyActivation(ctx, memberOperationInput(user.ID, queued.ID, purchaseops.OperationEarlyActivation, "recover-activate"), current.ID, queued.ID, now)
			if err != nil {
				t.Fatal(err)
			}
			worker := purchaseops.NewComboControlWorker(store, controlSyncFunc(func(ctx context.Context, _ model.OutboxJob) error {
				if err := store.AdvancePurchaseTrafficReset(ctx, queued.ID, "pending", "quiesced", now); err != nil {
					return err
				}
				if err := store.MarkPurchaseSyncResult(ctx, queued.ID, false, now); err != nil {
					return err
				}
				return errors.New("terminal constructed upstream outage")
			}), nil)
			if err := worker.HandleProviderOperation(ctx, operation, model.OutboxJob{Attempts: 10}); err == nil {
				t.Fatal("upstream failure ignored")
			}
			var jobID string
			if err := store.DB().QueryRowContext(ctx, `SELECT id FROM outbox_jobs WHERE kind='provider_operation' AND json_extract(payload,'$.operationId')=?`, operation.Receipt.ID).Scan(&jobID); err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE outbox_jobs SET status='failed',attempts=10,updated_at=? WHERE id=?`, stamp(now.Add(-3*24*time.Hour)), jobID); err != nil {
				t.Fatal(err)
			}
			before, err := store.Balance(ctx, user.ID)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "admin retry" {
				if err := store.RetryOutboxJob(ctx, jobID, now); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
					t.Fatal(err)
				}
			}
			recovered, err := store.ProviderOperationByID(ctx, operation.Receipt.ID)
			if err != nil || recovered.Receipt.Status != "queued" {
				t.Fatalf("receipt=%+v,%v", recovered, err)
			}
			phase, err := store.PurchaseTrafficResetPhase(ctx, queued.ID)
			if err != nil || phase != "quiesced" {
				t.Fatalf("phase=%s,%v", phase, err)
			}
			success := purchaseops.NewComboControlWorker(store, controlSyncFunc(func(ctx context.Context, _ model.OutboxJob) error {
				if err := store.AdvancePurchaseTrafficReset(ctx, queued.ID, "quiesced", "reset", now); err != nil {
					return err
				}
				return store.MarkPurchaseSyncResult(ctx, queued.ID, true, now)
			}), nil)
			if err := success.HandleProviderOperation(ctx, recovered, model.OutboxJob{Attempts: 1}); err != nil {
				t.Fatal(err)
			}
			active, err := store.PurchaseByID(ctx, queued.ID)
			if err != nil || active.Status != "active" || !active.ValidFrom.Equal(now) || !active.ValidUntil.Equal(now.Add(30*24*time.Hour)) {
				t.Fatalf("recovered dates=%+v,%v", active, err)
			}
			after, err := store.Balance(ctx, user.ID)
			if err != nil || after.Minor != before.Minor {
				t.Fatalf("recovery moved balance=%+v,%v", after, err)
			}
		})
	}
}

func TestComboControlRecoveryCannotRestoreExpiredCancelledOrSupersededAccess(t *testing.T) {
	for _, mode := range []string{"expired", "cancelled", "superseded"} {
		t.Run(mode, func(t *testing.T) {
			ctx, store := context.Background(), newTestStore(t)
			now := time.Now().UTC()
			combo := saveTestCombo(t, store, "stale-control", 1000, 30)
			user, current := createAdminWorkflowPurchase(t, store, 31941, combo, now)
			queued, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "stale-queued"}, now)
			if err != nil {
				t.Fatal(err)
			}
			operation, err := store.BeginEarlyActivation(ctx, memberOperationInput(user.ID, queued.ID, purchaseops.OperationEarlyActivation, "stale-activate"), current.ID, queued.ID, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE provider_operations SET status='failed' WHERE id=?`, operation.Receipt.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE purchases SET status='failed' WHERE id=?`, queued.ID); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "expired":
				_, err = store.DB().ExecContext(ctx, `UPDATE purchases SET valid_until=? WHERE id=?`, stamp(now), queued.ID)
			case "cancelled":
				_, err = store.DB().ExecContext(ctx, `UPDATE purchases SET status='cancelled' WHERE id=?`, queued.ID)
			case "superseded":
				_, err = store.DB().ExecContext(ctx, `UPDATE purchases SET status='active',valid_until=? WHERE id=?`, stamp(now.Add(24*time.Hour)), current.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			var jobID string
			if err := store.DB().QueryRowContext(ctx, `SELECT id FROM outbox_jobs WHERE kind='provider_operation' AND json_extract(payload,'$.operationId')=?`, operation.Receipt.ID).Scan(&jobID); err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE outbox_jobs SET status='failed' WHERE id=?`, jobID); err != nil {
				t.Fatal(err)
			}
			if err := store.RetryOutboxJob(ctx, jobID, now); err != nil {
				t.Fatal(err)
			}
			receipt, err := store.ProviderOperationByID(ctx, operation.Receipt.ID)
			if err != nil || receipt.Receipt.Status != "failed" {
				t.Fatalf("stale receipt reopened: %+v,%v", receipt, err)
			}
			if _, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
				t.Fatal(err)
			}
			if _, err := store.ProviderOperationByID(ctx, operation.Receipt.ID); !errors.Is(err, ErrNotFound) {
				t.Fatalf("stale receipt retained: %v", err)
			}
		})
	}
}
