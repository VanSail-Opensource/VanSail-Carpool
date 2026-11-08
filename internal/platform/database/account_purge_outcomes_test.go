package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestPurgePreservesAttemptedUnknownProviderContinuationAndFunds(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497851)
	now := time.Now().UTC()
	if _, err := s.AdjustBalance(ctx, u.ID, 500, "seed", "fixture", now); err != nil {
		t.Fatal(err)
	}
	op, _, err := s.CreateProviderOperation(ctx, cleanupOperationInput(u.ID), now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BeginProviderOperationAttempt(ctx, op.Receipt.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err = s.BeginProviderOperationItemAttempt(ctx, op.Receipt.ID, "one", now); err != nil {
		t.Fatal(err)
	}
	unknown := providerops.Completion{Status: providerops.StatusPendingReview, ErrorCode: "OUTCOME_UNKNOWN", ResultJSON: "{}"}
	if _, err = s.CompleteProviderOperationItem(ctx, op.Receipt.ID, "one", unknown, now); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CompleteProviderOperation(ctx, op.Receipt.ID, unknown, now); err != nil {
		t.Fatal(err)
	}
	admitTestAccountCleanup(t, s, u, now)
	if err = s.FenceAccountPurge(ctx, u.ID, 1, now); err != nil {
		t.Fatal(err)
	}
	dependency, err := s.AccountPurgeDependency(ctx, u.ID, 1, now)
	if err != nil || dependency != "account_purge_provider_outcome_pending" {
		t.Fatal("unknown provider outcome lost its dependency", dependency, err)
	}
	if err = s.CompleteAccountPurge(ctx, u.ID, 1, now); err == nil || !strings.Contains(err.Error(), dependency) {
		t.Fatal("unknown mutation permitted financial cleanup", err)
	}
	var status string
	var jobs int
	if err = s.DB().QueryRowContext(ctx, `SELECT status FROM provider_operations WHERE id=$1`, op.Receipt.ID).Scan(&status); err != nil || status != "pending_review" {
		t.Fatal("unknown receipt was relabeled", status, err)
	}
	if err = s.DB().QueryRowContext(ctx, `SELECT count(*) FROM outbox_jobs WHERE kind='provider_operation' AND payload::jsonb->>'operationId'=$1`, op.Receipt.ID).Scan(&jobs); err != nil || jobs != 1 {
		t.Fatal("unknown continuation was erased", jobs, err)
	}
	balance, err := s.Balance(ctx, u.ID)
	if err != nil || balance.MinorInt64() != 500 {
		t.Fatal("unresolved outcome changed the wallet", balance, err)
	}
}

func TestPurgeSupersedesOnlyNeverAttemptedIntents(t *testing.T) {
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497852)
	now := time.Now().UTC()
	op, _, err := s.CreateProviderOperation(ctx, cleanupOperationInput(u.ID), now)
	if err != nil {
		t.Fatal(err)
	}
	admitTestAccountCleanup(t, s, u, now)
	var status, code string
	var attempts int
	if err = s.DB().QueryRowContext(ctx, `SELECT status,error_code,attempts FROM provider_operations WHERE id=$1`, op.Receipt.ID).Scan(&status, &code, &attempts); err != nil || status != "failed" || code != "ACCOUNT_PURGED" || attempts != 0 {
		t.Fatal("never-attempted intent not safely superseded", status, code, attempts, err)
	}
	if err = s.CompleteAccountPurge(ctx, u.ID, 1, now); err != nil {
		t.Fatal("known unattempted receipt blocked cleanup", err)
	}
	var retained int
	if err = s.DB().QueryRowContext(ctx, `SELECT count(*) FROM provider_operations WHERE id=$1`, op.Receipt.ID).Scan(&retained); err != nil || retained != 1 {
		t.Fatal("idempotent receipt was deleted", retained, err)
	}
}
