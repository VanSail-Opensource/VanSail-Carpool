package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestCleanupAttemptedUnknownRetainsExistingOutboxContinuation(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497674)
	at := time.Now().UTC().Truncate(time.Microsecond)
	input := cleanupOperationInput(u.ID)
	input.SealedTarget = "sealed-unknown-target"
	op, _, err := s.CreateProviderOperation(ctx, input, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BeginProviderOperationAttempt(ctx, op.Receipt.ID, at); err != nil {
		t.Fatal(err)
	}
	var jobID, payload string
	var beforeAttempts int
	if err = s.DB().QueryRowContext(ctx, `SELECT id,payload,attempts FROM outbox_jobs WHERE kind='provider_operation'`).Scan(&jobID, &payload, &beforeAttempts); err != nil {
		t.Fatal(err)
	}
	admitTestAccountCleanup(t, s, u, at)
	if _, err = s.DB().ExecContext(ctx, `UPDATE outbox_jobs SET available_at=$1 WHERE kind='account_purge'`, stamp(at.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	called := false
	dispatcher := providerops.NewDispatcher(s)
	if err = dispatcher.Register(input.Kind, providerops.HandlerFunc(func(context.Context, providerops.Operation, model.OutboxJob) error { called = true; return nil })); err != nil {
		t.Fatal(err)
	}
	worker := outbox.NewWorker(s)
	if err = worker.Register(providerops.OutboxKind, dispatcher); err != nil {
		t.Fatal(err)
	}
	if err = worker.Drain(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("unknown external mutation replayed behind purge fence")
	}
	var status, afterPayload, lastError string
	var attempts int
	if err = s.DB().QueryRowContext(ctx, `SELECT status,payload,attempts,last_error FROM outbox_jobs WHERE id=$1`, jobID).Scan(&status, &afterPayload, &attempts, &lastError); err != nil || status != "pending" || attempts != beforeAttempts || afterPayload != payload || !strings.Contains(lastError, "account_operation_reconciliation_pending") {
		t.Fatal("continuation lost or exhausted", status, attempts, lastError, err)
	}
	stored, err := s.ProviderOperationByID(ctx, op.Receipt.ID)
	if err != nil || stored.Receipt.Status != "processing" || stored.Attempts != 1 {
		t.Fatal(stored, err)
	}
	if reason, err := s.AccountPurgeDependency(ctx, u.ID, 1, at); err != nil || reason != "account_purge_provider_outcome_pending" {
		t.Fatal("unknown dependency was hidden", reason, err)
	}
}
