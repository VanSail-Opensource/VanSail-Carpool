package database

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

func TestIPLookupGlobalAdmissionIsFIFOAcrossOwners(t *testing.T) {
	ctx := context.Background()
	store, service, user, _, now := ipLookupFixture(t, 0)
	first := submitIP(t, service, user, "oldest", false)
	other := createTestUser(t, store, 96002)
	q, err := service.Quote(ctx, other.ID, "1.1.1.1", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AdjustBalance(ctx, other.ID, 10000, "seed-other", "seed", now); err != nil {
		t.Fatal(err)
	}
	second, err := service.Submit(ctx, other.ID, "second", q)
	if err != nil {
		t.Fatal(err)
	}
	peer := NewStore(store.DB())
	if claimed, err := peer.ClaimIPLookupLease(ctx, second.ID, "later", now); err != nil || claimed {
		t.Fatalf("later report admitted first: %v %v", claimed, err)
	}
	if claimed, err := store.ClaimIPLookupLease(ctx, first.ID, "first", now); err != nil || !claimed {
		t.Fatalf("oldest not admitted: %v %v", claimed, err)
	}
	if claimed, err := peer.ClaimIPLookupLease(ctx, second.ID, "later", now.Add(61*time.Second)); err != nil || claimed {
		t.Fatalf("expiry broke FIFO: %v %v", claimed, err)
	}
	if claimed, err := peer.ClaimIPLookupLease(ctx, first.ID, "recovery", now.Add(61*time.Second)); err != nil || !claimed {
		t.Fatalf("oldest not recovered: %v %v", claimed, err)
	}
	pending, err := store.PendingIPLookupOperations(ctx)
	if err != nil || len(pending) != 1 || pending[0] != first.ID {
		t.Fatalf("wrong admission candidate: %v %v", pending, err)
	}
	check, err := service.Check(ctx, other.ID, second.ID)
	if err != nil || check.Operation.Status != "queued" || check.Progress.Phase != "queued" {
		t.Fatalf("waiting check attempted: %+v %v", check, err)
	}
	finishIP(t, store, first.ID, false)
	if claimed, err := peer.ClaimIPLookupLease(ctx, second.ID, "later", now.Add(62*time.Second)); err != nil || !claimed {
		t.Fatalf("next report not admitted: %v %v", claimed, err)
	}
}

func TestIPLookupSharedSubmissionIsFreeAcrossOwnersAndRaces(t *testing.T) {
	ctx := context.Background()
	store, service, user, _, _ := ipLookupFixture(t, 2)
	other := createTestUser(t, store, 96003)
	priced, err := service.Quote(ctx, other.ID, "8.8.8.8", false)
	if err != nil {
		t.Fatal(err)
	}
	first := submitIP(t, service, user, "initial", false)
	free, err := service.Quote(ctx, other.ID, "8.8.8.8", false)
	if err != nil || free.Charge.Minor != "0" || free.UseQuota || free.QuotaCost != 0 {
		t.Fatalf("shared quote not free: %+v %v", free, err)
	}
	joined, err := service.Submit(ctx, other.ID, "join-priced-race", priced)
	if err != nil {
		t.Fatal(err)
	}
	shared, _, err := store.IPLookupRun(ctx, joined.ID)
	original, _, originalErr := store.IPLookupRun(ctx, first.ID)
	if err != nil || originalErr != nil || shared.ID != original.ID {
		t.Fatalf("duplicated report: %v %v", err, originalErr)
	}
	if _, err := service.Check(ctx, user, joined.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner receipt exposed: %v", err)
	}
	finishIP(t, store, first.ID, false)
	check, err := service.Check(ctx, other.ID, joined.ID)
	if err != nil || check.Operation.Status != "succeeded" || check.Charge.Minor != "0" || check.UsedQuota || check.QuotaConsumed != 0 || check.Report.ID != original.ID {
		t.Fatalf("wrong shared settlement: %+v %v", check, err)
	}
	late, err := service.Submit(ctx, other.ID, "completed-race", free)
	if err != nil {
		t.Fatal(err)
	}
	check, err = service.Check(ctx, other.ID, late.ID)
	if err != nil || !check.Cached || check.Report.ID != original.ID || check.Charge.Minor != "0" {
		t.Fatalf("completion race recollected or charged: %+v %v", check, err)
	}
	replay, err := service.Submit(ctx, other.ID, "join-priced-race", priced)
	if err != nil || replay.ID != joined.ID {
		t.Fatalf("shared replay changed: %+v %v", replay, err)
	}
}

func TestIPLookupRepairsYieldToFreshChecksWithoutBurningAttempts(t *testing.T) {
	for _, kind := range []string{iplookup.ReportRetryOutboxKind, iplookup.LiveRetryOutboxKind} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			store, service, user, _, now := ipLookupFixture(t, 0)
			var retry any = iplookup.ReportRetryPayload{ReportID: "frozen-report", Attempt: 1}
			if kind == iplookup.LiveRetryOutboxKind {
				retry = iplookup.LiveRetryPayload{SnapshotID: "frozen-snapshot", Attempt: 1}
			}
			payload, err := json.Marshal(retry)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.DB().ExecContext(ctx, `DELETE FROM outbox_jobs`); err != nil {
				t.Fatal(err)
			}
			if err := store.EnqueueOutbox(ctx, kind, string(payload), now); err != nil {
				t.Fatal(err)
			}
			repair, err := store.ClaimOutboxJob(ctx, now.Add(time.Second))
			if err != nil || repair == nil || repair.Kind != kind {
				t.Fatalf("repair not admitted: %+v %v", repair, err)
			}
			fresh := submitIP(t, service, user, "during-repair", false)
			if claimed, err := store.ClaimIPLookupLease(ctx, fresh.ID, "fresh", now); err != nil || claimed {
				t.Fatalf("fresh overlapped repair: %v %v", claimed, err)
			}
			if err := store.CompleteOutboxClaim(ctx, *repair, nil, now); err != nil {
				t.Fatal(err)
			}
			if claimed, err := store.ClaimIPLookupLease(ctx, fresh.ID, "fresh", now); err != nil || !claimed {
				t.Fatalf("fresh not resumed: %v %v", claimed, err)
			}
			if _, err := store.DB().ExecContext(ctx, `DELETE FROM outbox_jobs`); err != nil {
				t.Fatal(err)
			}
			if err := store.EnqueueOutbox(ctx, kind, string(payload), now); err != nil {
				t.Fatal(err)
			}
			if job, err := store.ClaimOutboxJob(ctx, now.Add(time.Second)); err != nil || job != nil {
				t.Fatalf("repair admitted during fresh work: %+v %v", job, err)
			}
			var attempts int
			if err := store.DB().QueryRowContext(ctx, `SELECT attempts FROM outbox_jobs WHERE kind=?`, kind).Scan(&attempts); err != nil || attempts != 0 {
				t.Fatalf("waiting burned attempt: %d %v", attempts, err)
			}
		})
	}
}
