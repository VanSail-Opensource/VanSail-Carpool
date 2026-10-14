package database

import (
	"context"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

func TestCombinedRefreshFromSubnetChargesOnceAndRecoversSnapshot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, _, now := ipLookupFixture(t, 2)
	baseline := submitIP(t, service, user, "baseline", false)
	finishIP(t, store, baseline.ID, false)
	q, err := service.Quote(ctx, user, "8.8.8.9", true)
	if err != nil || q.Charge.Minor != "350" || q.UseQuota {
		t.Fatalf("refresh quote: %+v %v", q, err)
	}
	before, err := store.Balance(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	op, err := service.Submit(ctx, user, "combined", q)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := service.Submit(ctx, user, "combined", q)
	if err != nil || replay.ID != op.ID {
		t.Fatal("replay changed operation")
	}
	r, _, err := store.IPLookupRun(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	stage, claimed, err := store.ClaimIPLookupLiveStage(ctx, r.ID)
	if err != nil || !claimed {
		t.Fatal("live phase not claimed")
	}
	if err := store.MarkIPLookupLiveAttempt(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	id, err := store.SaveIPDetails(ctx, r.ID, iplookup.LiveDetails{IP: q.IP, FetchedAt: r.CheckedAt}, now)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate restart after snapshot storage but before phase completion.
	stage, claimed, err = NewStore(store.DB()).ClaimIPLookupLiveStage(ctx, r.ID)
	if err != nil || claimed || stage.SnapshotID != id || !stage.Attempted || stage.Status != "succeeded" {
		t.Fatalf("failed recovery: %+v %v", stage, err)
	}
	finishIP(t, store, op.ID, false)
	check, err := service.Check(ctx, user, op.ID)
	if err != nil || check.LiveDetails == nil || check.LiveDetails.SourceIP != q.IP || check.LiveDetails.IP != q.IP || check.Charge.Minor != "350" || check.UsedQuota {
		t.Fatalf("combined receipt: %+v %v", check, err)
	}
	after, err := store.Balance(ctx, user)
	if err != nil || before.MinorInt64()-after.MinorInt64() != 350 {
		t.Fatal("refresh charged more than once")
	}
	state, err := service.State(ctx, user)
	if err != nil || state.Allowance.Remaining != 1 {
		t.Fatal("refresh consumed quota")
	}
}

func TestLiveOnlyAttemptSurvivesStalledRunMaintenanceWithoutRefund(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, _, now := ipLookupFixture(t, 0)
	baseline := submitIP(t, service, user, "baseline", false)
	finishIP(t, store, baseline.ID, false)
	op := submitIP(t, service, user, "stalled-live", true)
	if _, err := store.DB().ExecContext(ctx, `UPDATE ip_lookup_reports SET lease_owner='interrupted',heartbeat_at='2000-01-01T00:00:00Z',lease_expires_at='2000-01-01T00:01:00Z' WHERE id=(SELECT report_id FROM ip_lookup_checks WHERE operation_id=?)`, op.ID); err != nil {
		t.Fatal(err)
	}
	r, _, err := store.IPLookupRun(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ClaimIPLookupLiveStage(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkIPLookupLiveAttempt(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE maintenance_operation_candidates(id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO maintenance_operation_candidates(id) VALUES(?)`, op.ID); err != nil {
		t.Fatal(err)
	}
	if err := reconcileStaleIPLookupsTx(ctx, tx, now); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE maintenance_operation_candidates`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	check, err := service.Check(ctx, user, op.ID)
	if err != nil || check.Refunded || check.Charge.Minor != "350" || check.Operation.Status != "partial" {
		t.Fatalf("lost live attempt: %+v %v", check, err)
	}
}
