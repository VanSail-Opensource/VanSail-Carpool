package database

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

func TestIPLookupExpiredWorkerCannotPublishOrRefundAfterTakeover(t *testing.T) {
	for _, takeover := range []bool{false, true} {
		t.Run(map[bool]string{false: "expired", true: "replaced"}[takeover], func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			store, service, user, _, now := ipLookupFixture(t, 0)
			baseline := submitIP(t, service, user, "lease-baseline", false)
			finishIP(t, store, baseline.ID, false)
			op := submitIP(t, service, user, "lease-fence", true)
			old := iplookup.WithRunLease(ctx, "old-execution")
			current := iplookup.WithRunLease(ctx, "new-execution")
			if claimed, err := store.ClaimIPLookupLease(ctx, op.ID, "old-execution", time.Now().UTC()); err != nil || !claimed {
				t.Fatalf("initial claim: %v %v", claimed, err)
			}
			r, _, err := store.IPLookupRun(ctx, op.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, claimed, err := store.ClaimIPLookupLiveStage(old, r.ID); err != nil || !claimed {
				t.Fatalf("initial live claim: %v %v", claimed, err)
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE ip_lookup_reports SET lease_expires_at=$1 WHERE id=$2`, stamp(now.Add(-time.Second)), r.ID); err != nil {
				t.Fatal(err)
			}
			if err := store.HeartbeatIPLookup(ctx, op.ID, "old-execution", now); !errors.Is(err, ErrConflict) {
				t.Fatalf("expired heartbeat revived the lease: %v", err)
			}
			claimCurrent := func() {
				t.Helper()
				if claimed, err := NewStore(store.DB()).ClaimIPLookupLease(ctx, op.ID, "new-execution", time.Now().UTC()); err != nil || !claimed {
					t.Fatalf("takeover: %v %v", claimed, err)
				}
			}
			if takeover {
				claimCurrent()
			}
			before, err := store.Balance(ctx, user)
			if err != nil {
				t.Fatal(err)
			}
			failed := r
			failed.Status, failed.Verdict, failed.RefundRequired = "failed", "inconclusive", true
			failed.Checkpoint = nil
			for _, stale := range []context.Context{old, ctx} {
				checks := map[string]func() error{
					"progress": func() error { return store.SaveIPLookupProgress(stale, r) },
					"finish":   func() error { return store.FinishIPLookupRun(stale, failed, now) },
					"claim live": func() error {
						_, _, err := store.ClaimIPLookupLiveStage(stale, r.ID)
						return err
					},
					"attempt live": func() error { return store.MarkIPLookupLiveAttempt(stale, r.ID) },
					"finish live": func() error {
						return store.FinishIPLookupLiveStage(stale, r.ID, iplookup.LiveStage{Status: "error"})
					},
					"snapshot": func() error {
						_, err := store.SaveIPDetails(stale, r.ID, iplookup.LiveDetails{IP: r.IP, SourceIP: r.IP}, now)
						return err
					},
				}
				for name, call := range checks {
					if err := call(); !errors.Is(err, ErrConflict) {
						t.Fatalf("stale %s accepted: %v", name, err)
					}
				}
			}
			var status, live string
			var snapshots int
			if err := store.DB().QueryRowContext(ctx, `SELECT status,live_state_json,(SELECT COUNT(*) FROM ip_lookup_live_snapshots WHERE run_key=$1) FROM ip_lookup_reports WHERE id=$1`, r.ID).Scan(&status, &live, &snapshots); err != nil || status != "processing" || snapshots != 0 {
				t.Fatalf("stale execution changed evidence: %s %s %d %v", status, live, snapshots, err)
			}
			var stage iplookup.LiveStage
			if err := json.Unmarshal([]byte(live), &stage); err != nil || stage.Status != "processing" || stage.Attempted || stage.SnapshotID != "" {
				t.Fatalf("stale execution changed live stage: %+v %v", stage, err)
			}
			if !takeover {
				claimCurrent()
			}
			// The replacement records a real attempted provider stage. The old
			// pre-attempt snapshot must never turn that retained cost into a refund.
			for i := range r.Checkpoint.Stages {
				if r.Checkpoint.Stages[i].ID == "ipapi" {
					r.Checkpoint.Stages[i].Status, r.Checkpoint.Stages[i].Attempted = "error", true
				}
			}
			r.Facts.Country = "NZ"
			if err := store.SaveIPLookupProgress(current, r); err != nil {
				t.Fatal(err)
			}
			if _, _, err := store.ClaimIPLookupLiveStage(current, r.ID); err != nil {
				t.Fatal(err)
			}
			if err := store.SaveIPLookupProgress(old, failed); !errors.Is(err, ErrConflict) {
				t.Fatalf("old snapshot overwrote newer attempted progress: %v", err)
			}
			if err := store.FinishIPLookupRun(old, failed, now); !errors.Is(err, ErrConflict) {
				t.Fatalf("old pre-attempt snapshot refunded the current execution: %v", err)
			}
			r.Status = "failed"
			if err := store.FinishIPLookupRun(current, r, now); err != nil {
				t.Fatal(err)
			}
			// Terminal replay reads the winner's evidence even from the former
			// context, and may not replace it with the losing failed projection.
			if err := store.FinishIPLookupRun(old, failed, now); err != nil {
				t.Fatal(err)
			}
			check, err := service.Check(ctx, user, op.ID)
			if err != nil || check.Refunded || check.Operation.Status != "partial" || check.Report == nil || check.Report.Facts.Country != "NZ" {
				t.Fatalf("wrong winner settlement: %+v %v", check, err)
			}
			after, err := store.Balance(ctx, user)
			if err != nil || after.Minor != before.Minor {
				t.Fatalf("stale worker changed debit: before=%+v after=%+v err=%v", before, after, err)
			}
		})
	}
}
