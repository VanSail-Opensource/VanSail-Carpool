package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestMaintenancePreservesUncertainReceiptsAndEncryptedContinuations(t *testing.T) {
	t.Parallel()
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497681)
	now := time.Now().UTC().Truncate(time.Microsecond)
	old := now.Add(-40 * 24 * time.Hour)
	type retained struct{ id, payload string }
	var keep []retained
	var remove []string
	for _, c := range []struct {
		name, status, item, code, job string
		attempts                      int
		retain                        bool
	}{
		{"running", "processing", "processing", "", "pending", 1, true},
		{"exhausted", "processing", "processing", "", "failed", 1, true},
		{"review", "pending_review", "pending_review", "UNKNOWN", "failed", 1, true},
		{"partial", "partial", "pending_review", "UNKNOWN", "failed", 1, true},
		{"child-only", "queued", "processing", "", "pending", 0, true},
		{"failed-parent", "failed", "processing", "PROVIDER_FAILED", "failed", 1, true},
		{"old-timeout", "failed", "failed", maintenanceTimeoutCode, "failed", 1, true},
		{"old-child-timeout", "failed", "failed", maintenanceTimeoutCode, "failed", 0, true},
		{"never-started", "queued", "queued", "", "failed", 0, false},
		{"confirmed-failed", "failed", "failed", "REJECTED", "failed", 1, false},
		{"confirmed-success", "succeeded", "succeeded", "", "failed", 1, false},
	} {
		op, _, err := s.CreateProviderOperation(ctx, providerops.CreateInput{
			ActorUserID: u.ID, OwnerUserID: u.ID, Kind: "retention-proof", IdempotencyKey: c.name,
			RequestFingerprint: "retention-proof-" + c.name, SealedTarget: "synthetic-encrypted-" + c.name,
			Items: []providerops.ItemInput{{Key: "target", TargetType: "user", TargetID: u.ID}},
		}, old)
		if err != nil {
			t.Fatal(err)
		}
		var started any
		if c.attempts > 0 {
			started = stamp(old)
		}
		if _, err = s.DB().ExecContext(ctx, `UPDATE provider_operations SET status=$2,attempts=$3,attempt_started_at=$4,error_code=$5 WHERE id=$1`, op.Receipt.ID, c.status, c.attempts, started, c.code); err != nil {
			t.Fatal(err)
		}
		if _, err = s.DB().ExecContext(ctx, `UPDATE provider_operation_items SET status=$2,attempt_started_at=CASE WHEN $2 IN('processing','pending_review','partial') THEN $3 ELSE NULL END WHERE operation_id=$1`, op.Receipt.ID, c.item, stamp(old)); err != nil {
			t.Fatal(err)
		}
		if c.name == "old-child-timeout" {
			if _, err = s.DB().ExecContext(ctx, `UPDATE provider_operation_items SET attempt_started_at=$2 WHERE operation_id=$1`, op.Receipt.ID, stamp(old)); err != nil {
				t.Fatal(err)
			}
		}
		var payload string
		if err = s.DB().QueryRowContext(ctx, `UPDATE outbox_jobs SET status=$2,attempts=10,updated_at=$3 WHERE payload::jsonb->>'operationId'=$1 RETURNING payload`, op.Receipt.ID, c.job, stamp(old)).Scan(&payload); err != nil {
			t.Fatal(err)
		}
		if c.retain {
			keep = append(keep, retained{op.Receipt.ID, payload})
		} else {
			remove = append(remove, op.Receipt.ID)
		}
	}
	for range 2 {
		if _, err := s.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
			t.Fatal(err)
		}
		for _, entry := range keep {
			if _, err := s.ProviderOperationByID(ctx, entry.id); err != nil {
				t.Fatal("uncertain receipt was age-deleted", err)
			}
			var payload string
			if err := s.DB().QueryRowContext(ctx, `SELECT payload FROM outbox_jobs WHERE kind='provider_operation' AND payload::jsonb->>'operationId'=$1`, entry.id).Scan(&payload); err != nil || payload != entry.payload || !strings.Contains(payload, "synthetic-encrypted-") {
				t.Fatal("encrypted continuation was discarded", err)
			}
		}
		for _, id := range remove {
			if _, err := s.ProviderOperationByID(ctx, id); err != nil {
				t.Fatal("known settled/unattempted replay receipt was deleted", err)
			}
			var jobs int
			if err := s.DB().QueryRowContext(ctx, `SELECT count(*) FROM outbox_jobs WHERE kind='provider_operation' AND payload::jsonb->>'operationId'=$1`, id).Scan(&jobs); err != nil || jobs != 0 {
				t.Fatal("settled encrypted continuation was not compacted", jobs, err)
			}
		}
	}
}

func TestMaintenanceNeverRefundsAnUncertainLegacyReset(t *testing.T) {
	t.Parallel()
	ctx, s := context.Background(), newTestStore(t)
	u := createTestUser(t, s, 497682)
	now := time.Now().UTC()
	old := now.Add(-40 * 24 * time.Hour)
	if _, err := s.AdjustBalance(ctx, u.ID, 1000, "retention-uncertain-seed", "seed", old); err != nil {
		t.Fatal(err)
	}
	makeReset := func(key string, price int64) providerops.Operation {
		return seedLegacyTrafficReset(t, s, providerops.CreateInput{ActorUserID: u.ID, OwnerUserID: u.ID,
			Kind: "purchase_traffic_reset", IdempotencyKey: key, RequestFingerprint: "retention-" + key,
			SealedTarget: "synthetic-encrypted-reset", Items: []providerops.ItemInput{{Key: "reset", TargetType: "user", TargetID: u.ID}},
		}, price, old)
	}
	unknown := makeReset("unknown-reset", 125)
	known := makeReset("never-started-reset", 25)
	if _, err := s.BeginProviderOperationAttempt(ctx, unknown.Receipt.ID, old); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := s.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
			t.Fatal(err)
		}
		balance, err := s.Balance(ctx, u.ID)
		if err != nil || balance.Minor != "875" {
			t.Fatal("age inferred a refund or repeated a known refund", balance.Minor, err)
		}
		var unknownRefund, knownRefund int
		if err = s.DB().QueryRowContext(ctx, `SELECT count(*) FILTER(WHERE reference_id=$1),count(*) FILTER(WHERE reference_id=$2) FROM ledger_entries WHERE kind='traffic_reset_compensation'`, unknown.Receipt.ID, known.Receipt.ID).Scan(&unknownRefund, &knownRefund); err != nil || unknownRefund != 0 || knownRefund != 1 {
			t.Fatal("reset compensation lost exact outcome distinction", unknownRefund, knownRefund, err)
		}
		if _, err = s.ProviderOperationByID(ctx, unknown.Receipt.ID); err != nil {
			t.Fatal("uncertain debit lost its receipt", err)
		}
	}
}
