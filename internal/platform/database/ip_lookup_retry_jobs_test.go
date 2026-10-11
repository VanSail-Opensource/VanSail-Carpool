package database

import (
	"context"
	"testing"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
)

// freezePartialRun drives one submitted run into a frozen partial report with
// a failed provider stage, mirroring the outage fixture flow.
func freezePartialRun(t *testing.T, store *Store, service *iplookup.Service, user, key string, status string) iplookup.Report {
	t.Helper()
	ctx := context.Background()
	op := submitIP(t, service, user, key, false)
	r, _, err := store.IPLookupRun(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := range r.Checkpoint.Stages {
		if r.Checkpoint.Stages[i].Status == "queued" {
			r.Checkpoint.Stages[i].Status = "error"
			r.Checkpoint.Stages[i].Attempted = true
			break
		}
	}
	if status == "partial" {
		r.Status, r.Verdict = "partial", "inconclusive"
	} else {
		r.Status, r.Verdict = "succeeded", "suitable"
	}
	r.CheckedAt = time.Now().UTC().Format(time.RFC3339Nano)
	stage, claimed, err := store.ClaimIPLookupLiveStage(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if claimed {
		details := iplookup.LiveDetails{IP: r.IP, FetchedAt: r.CheckedAt, Topology: iplookup.BGPTopology{LiveMeta: iplookup.LiveMeta{Status: "empty"}}}
		stage.SnapshotID, err = store.SaveIPDetails(ctx, r.ID, details, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		stage.Status = "succeeded"
		if err := store.FinishIPLookupLiveStage(ctx, r.ID, stage); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.FinishIPLookupRun(ctx, r, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	return r
}

func outboxRetryRows(t *testing.T, store *Store, kind string) []struct {
	payload     string
	availableAt string
} {
	t.Helper()
	rows, err := store.DB().QueryContext(context.Background(), `SELECT payload,available_at FROM outbox_jobs WHERE kind=? ORDER BY created_at`, kind)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result []struct {
		payload     string
		availableAt string
	}
	for rows.Next() {
		var payload, availableAt string
		if err := rows.Scan(&payload, &availableAt); err != nil {
			t.Fatal(err)
		}
		result = append(result, struct {
			payload     string
			availableAt string
		}{payload, availableAt})
	}
	return result
}

func TestFreezingPartialReportSchedulesDelayedFirstRetry(t *testing.T) {
	t.Parallel()
	store, service, user, _, _ := ipLookupFixture(t, 0)
	before := time.Now().UTC().Truncate(time.Second)
	r := freezePartialRun(t, store, service, user, "retry-first", "partial")
	jobs := outboxRetryRows(t, store, iplookup.ReportRetryOutboxKind)
	if len(jobs) != 1 {
		t.Fatalf("expected one scheduled retry, got %d", len(jobs))
	}
	job := jobs[0]
	if job.payload != `{"attempt":1,"reportId":"`+r.ID+`"}` && job.payload != `{"reportId":"`+r.ID+`","attempt":1}` {
		t.Fatalf("wrong payload: %s", job.payload)
	}
	available, err := time.Parse(time.RFC3339Nano, job.availableAt)
	if err != nil {
		t.Fatal(err)
	}
	if available.Before(before.Add(59*time.Second)) || available.After(before.Add(61*time.Second)) {
		t.Fatalf("retry not delayed by the default minute: %v", job.availableAt)
	}
}

func TestFreezingSucceededReportSchedulesNoRetry(t *testing.T) {
	t.Parallel()
	store, service, user, _, _ := ipLookupFixture(t, 0)
	freezePartialRun(t, store, service, user, "retry-none", "succeeded")
	if jobs := outboxRetryRows(t, store, iplookup.ReportRetryOutboxKind); len(jobs) != 0 {
		t.Fatalf("succeeded run scheduled retries: %d", len(jobs))
	}
}

func TestCachedReportRepairKeepsCompletionAndStopsAfterStatusChange(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _, _, _, completed := ipLookupFixture(t, 1)
	r := insertIPCache(t, store, "repair-target", "8.8.8.8", "partial", completed)
	loaded, config, err := store.IPLookupReportByID(ctx, "repair-target")
	if err != nil || loaded.ID != r.ID || !iplookup.RetryAllowed(config, 1) {
		t.Fatalf("load failed: %+v %v", loaded, err)
	}
	if _, _, err := store.IPLookupReportByID(ctx, "missing"); err != iplookup.ErrRepairGone {
		t.Fatalf("missing row not gone: %v", err)
	}
	// Re-freeze the cached row with a transport-failed stage to prove the
	// codec round-trip keeps failed stages repairable.
	for i := range loaded.Checkpoint.Stages {
		if loaded.Checkpoint.Stages[i].Status == "disabled" {
			loaded.Checkpoint.Stages[i].Status = "error"
			loaded.Checkpoint.Stages[i].Attempted = true
			break
		}
	}
	raw, err := iplookup.EncodeReport(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE ip_lookup_reports SET report_json=? WHERE id=?`, string(raw), "repair-target"); err != nil {
		t.Fatal(err)
	}
	loaded, _, err = store.IPLookupReportByID(ctx, "repair-target")
	if err != nil {
		t.Fatal(err)
	}
	if failed := iplookup.FailedStages(loaded); len(failed) != 1 {
		t.Fatalf("cached partial row exposes no failed stages: %v", failed)
	}
	iplookup.PrepareRepair(&loaded)
	loaded.Status = "succeeded"
	if err := store.UpdateIPLookupCachedReport(ctx, loaded); err != nil {
		t.Fatal(err)
	}
	var status, completedAt string
	if err := store.DB().QueryRowContext(ctx, `SELECT status,completed_at FROM ip_lookup_reports WHERE id=?`, "repair-target").Scan(&status, &completedAt); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" || completedAt != stamp(completed) {
		t.Fatalf("repair rewrote freeze data: status=%s completed=%s", status, completedAt)
	}
	if err := store.UpdateIPLookupCachedReport(ctx, loaded); err != iplookup.ErrRepairConflict {
		t.Fatalf("second repair not conflicting: %v", err)
	}
}

func TestSavingPartialLiveSnapshotSchedulesSectionRetry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _, _, _, now := ipLookupFixture(t, 1)
	healthy := iplookup.LiveDetails{IP: "1.1.1.1", FetchedAt: now.Format(time.RFC3339Nano), Topology: iplookup.BGPTopology{LiveMeta: iplookup.LiveMeta{Status: "empty"}}}
	if _, err := store.SaveIPDetails(ctx, "", healthy, now); err != nil {
		t.Fatal(err)
	}
	partial := healthy
	partial.IP = "1.1.1.2"
	partial.PTR = iplookup.PTRDetails{LiveMeta: iplookup.LiveMeta{Status: "error"}}
	snapshotID, err := store.SaveIPDetails(ctx, "", partial, now)
	if err != nil {
		t.Fatal(err)
	}
	jobs := outboxRetryRows(t, store, iplookup.LiveRetryOutboxKind)
	if len(jobs) != 1 {
		t.Fatalf("expected one live retry, got %d", len(jobs))
	}
	job := jobs[0]
	if job.payload != `{"attempt":1,"snapshotId":"`+snapshotID+`"}` && job.payload != `{"snapshotId":"`+snapshotID+`","attempt":1}` {
		t.Fatalf("wrong payload: %s", job.payload)
	}
	available, err := time.Parse(time.RFC3339Nano, job.availableAt)
	if err != nil {
		t.Fatal(err)
	}
	if available.Before(now.Truncate(time.Second)) {
		t.Fatalf("live retry not delayed: %v", job.availableAt)
	}
	details, status, err := store.IPLiveSnapshotByID(ctx, snapshotID)
	if err != nil || status != "partial" || details.IP != "1.1.1.2" {
		t.Fatalf("snapshot load failed: %+v %s %v", details, status, err)
	}
	if err := store.UpdateIPLiveSnapshot(ctx, snapshotID, healthy); err != nil {
		t.Fatal(err)
	}
	if _, status, err := store.IPLiveSnapshotByID(ctx, snapshotID); err != nil || status != "succeeded" {
		t.Fatalf("repaired snapshot status=%s %v", status, err)
	}
}
