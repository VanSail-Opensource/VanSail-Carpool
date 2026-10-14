package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

func TestIPLookupMaintenancePreservesReceiptsAndRefundsStalledRuns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, _, now := ipLookupFixture(t, 0)
	before, err := store.Balance(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	op := submitIP(t, service, user, "stalled", false)
	if _, err := store.DB().ExecContext(ctx, `UPDATE ip_lookup_reports SET lease_owner='interrupted',heartbeat_at=?,lease_expires_at=? WHERE id=(SELECT report_id FROM ip_lookup_checks WHERE operation_id=?)`, stamp(now.Add(-48*time.Hour)), stamp(now.Add(-47*time.Hour)), op.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE provider_operations SET created_at=? WHERE id=?`, stamp(now.Add(-48*time.Hour)), op.ID); err != nil {
		t.Fatal(err)
	}
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := pruneProviderOperationsTx(ctx, tx, now.Add(-24*time.Hour), now, map[string]int64{}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	check, err := service.Check(ctx, user, op.ID)
	if err != nil || !check.Refunded || check.Operation.Status != "failed" {
		t.Fatalf("maintenance lost receipt/refund=%+v %v", check, err)
	}
	after, err := store.Balance(ctx, user)
	if err != nil || after.Minor != before.Minor {
		t.Fatal("maintenance stranded debit")
	}
	q, err := service.Quote(ctx, user, "8.8.8.8", false)
	if err != nil {
		t.Fatal(err)
	}
	q.ExpiresAt = time.Now().Add(time.Minute).Unix()
	if q.CacheReportID != "" {
		t.Fatal("stalled empty report became cache")
	}
	// A new command must be able to reserve another run for the same IP.
	if _, err := store.CreateIPLookupCheck(ctx, user, "replacement", q, now); err != nil {
		t.Fatal(err)
	}
}

func TestIPLookupCompletedReceiptSurvivesMaintenance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, _, now := ipLookupFixture(t, 0)
	op := submitIP(t, service, user, "complete", false)
	finishIP(t, store, op.ID, false)
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := pruneProviderOperationsTx(ctx, tx, now.Add(-24*time.Hour), now, map[string]int64{}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	check, err := service.Check(ctx, user, op.ID)
	if err != nil || check.Report == nil {
		t.Fatalf("completed receipt deleted: %+v %v", check, err)
	}
}

func TestIPLookupMaintenanceCachesAttemptedProviderOutageWithoutRefund(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, service, user, _, now := ipLookupFixture(t, 0)
	op := submitIP(t, service, user, "attempted-stalled", false)
	if _, err := store.DB().ExecContext(ctx, `UPDATE ip_lookup_reports SET lease_owner='interrupted',heartbeat_at=?,lease_expires_at=? WHERE id=(SELECT report_id FROM ip_lookup_checks WHERE operation_id=?)`, stamp(now.Add(-48*time.Hour)), stamp(now.Add(-47*time.Hour)), op.ID); err != nil {
		t.Fatal(err)
	}
	report, _, err := store.IPLookupRun(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := range report.Checkpoint.Stages {
		if report.Checkpoint.Stages[i].ID == "ipapi" {
			report.Checkpoint.Stages[i].Status = "processing"
			report.Checkpoint.Stages[i].Attempted = true
		}
	}
	if err := store.SaveIPLookupProgress(ctx, report); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE provider_operations SET created_at=? WHERE id=?`, stamp(now.Add(-48*time.Hour)), op.ID); err != nil {
		t.Fatal(err)
	}
	before, err := store.Balance(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := pruneProviderOperationsTx(ctx, tx, now.Add(-24*time.Hour), now, map[string]int64{}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	check, err := service.Check(ctx, user, op.ID)
	if err != nil || check.Refunded || check.Operation.Status != "partial" || check.Report == nil || check.Charge.Minor != "250" {
		t.Fatalf("attempted maintenance=%+v %v", check, err)
	}
	after, err := store.Balance(ctx, user)
	if err != nil || after.Minor != before.Minor {
		t.Fatal("attempted outage was refunded")
	}
	preview, err := service.PreviewQuote(ctx, user, "8.8.8.9", false)
	if err != nil || preview.CacheMatch != "subnet" || preview.CachedReport == nil || !iplookup.ReportAttempted(*check.Report) {
		t.Fatalf("attempted outage not cacheable: %+v %v", preview, err)
	}
}
