package iplookup

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type enqueuedRetry struct {
	kind    string
	payload string
	at      time.Time
}

type reportRetryRepo struct {
	workerRepo
	gone     bool
	conflict bool
	enqueued []enqueuedRetry
}

func (r *reportRetryRepo) IPLookupReportByID(context.Context, string) (Report, Config, error) {
	if r.gone {
		return Report{}, Config{}, ErrRepairGone
	}
	return r.report, r.config, nil
}

func (r *reportRetryRepo) UpdateIPLookupCachedReport(_ context.Context, report Report) error {
	if r.conflict {
		return ErrRepairConflict
	}
	r.report, r.saves = report, r.saves+1
	return nil
}

func (r *reportRetryRepo) EnqueueOutbox(_ context.Context, kind, payload string, at time.Time) error {
	r.enqueued = append(r.enqueued, enqueuedRetry{kind: kind, payload: payload, at: at})
	return nil
}

// partialReport freezes one successful stage and one failed stage.
func partialReport(t *testing.T, c Config) Report {
	t.Helper()
	r := NewReport("report", "150.249.241.62", c)
	r.Checkpoint.RPKI.Status = "success"
	RecordProvider(&r, ProviderResult{ID: "abuseipdb", Status: "success", Complete: true}, c)
	RecordProvider(&r, ProviderResult{ID: "scamalytics", Status: "error"}, c)
	r = Aggregate(r, time.Now().UTC())
	if r.Status != "partial" {
		t.Fatalf("expected partial freeze, got %s", r.Status)
	}
	return r
}

func reportRetryJob(t *testing.T, id string, attempt int) model.OutboxJob {
	t.Helper()
	payload, err := json.Marshal(ReportRetryPayload{ReportID: id, Attempt: attempt})
	if err != nil {
		t.Fatal(err)
	}
	return model.OutboxJob{Payload: string(payload)}
}

func TestReportRetryRepairsFailedStageWithoutReplayingSuccess(t *testing.T) {
	t.Parallel()
	c := DefaultConfig()
	for i := range c.Providers {
		c.Providers[i].Enabled = c.Providers[i].ID == "abuseipdb" || c.Providers[i].ID == "scamalytics"
	}
	repo := &reportRetryRepo{workerRepo: workerRepo{config: c}}
	repo.report = partialReport(t, c)
	done := &fakeProvider{id: "scamalytics", result: ProviderResult{ID: "scamalytics", Status: "success", Complete: true, Facts: Facts{Country: "JP"}}}
	w := NewWorker(repo, []Provider{done})
	if err := w.HandleReportRetryOutbox(context.Background(), reportRetryJob(t, "report", 1)); err != nil {
		t.Fatal(err)
	}
	if done.calls != 1 || len(FailedStages(repo.report)) != 0 || len(repo.enqueued) != 0 {
		t.Fatalf("repair incomplete: calls=%d failed=%d enqueued=%d", done.calls, len(FailedStages(repo.report)), len(repo.enqueued))
	}
	// Coverage may keep the report partial, but every provider stage must be healthy.
	for _, stage := range repo.report.Checkpoint.Stages {
		if stage.Status != "success" && stage.Status != "disabled" {
			t.Fatalf("stage not repaired: %+v", stage)
		}
	}
}

func TestReportRetryChainsNextAttemptAfterConfiguredDelay(t *testing.T) {
	t.Parallel()
	c := DefaultConfig()
	c.Retry = &RetryPolicy{Enabled: true, MaxAttempts: 3, DelaySeconds: 120}
	for i := range c.Providers {
		c.Providers[i].Enabled = c.Providers[i].ID == "abuseipdb" || c.Providers[i].ID == "scamalytics"
	}
	repo := &reportRetryRepo{workerRepo: workerRepo{config: c}}
	repo.report = partialReport(t, c)
	stillFailing := &fakeProvider{id: "scamalytics", err: context.Canceled}
	w := NewWorker(repo, []Provider{stillFailing})
	fixed := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	w.now = func() time.Time { return fixed }
	if err := w.HandleReportRetryOutbox(context.Background(), reportRetryJob(t, "report", 1)); err != nil {
		t.Fatal(err)
	}
	if len(repo.enqueued) != 1 {
		t.Fatalf("expected one chained job, got %d", len(repo.enqueued))
	}
	next := repo.enqueued[0]
	var payload ReportRetryPayload
	if next.kind != ReportRetryOutboxKind || next.at != fixed.Add(2*time.Minute) || json.Unmarshal([]byte(next.payload), &payload) != nil || payload.Attempt != 2 || payload.ReportID != "report" {
		t.Fatalf("wrong chained job: %+v %s", next, next.payload)
	}
}

func TestReportRetryStopsAtPolicyBoundaryAndTerminalStates(t *testing.T) {
	t.Parallel()
	c := DefaultConfig()
	for i := range c.Providers {
		c.Providers[i].Enabled = c.Providers[i].ID == "abuseipdb" || c.Providers[i].ID == "scamalytics"
	}
	failing := &fakeProvider{id: "scamalytics", err: context.Canceled}
	w := NewWorker(&reportRetryRepo{workerRepo: workerRepo{config: c}}, []Provider{failing})

	// The final allowed attempt repairs but must not schedule attempt 4.
	repo := &reportRetryRepo{workerRepo: workerRepo{config: c}}
	repo.report = partialReport(t, c)
	w.repository = repo
	if err := w.HandleReportRetryOutbox(context.Background(), reportRetryJob(t, "report", 3)); err != nil {
		t.Fatal(err)
	}
	if failing.calls != 1 || len(repo.enqueued) != 0 {
		t.Fatalf("boundary attempt chained: calls=%d enqueued=%d", failing.calls, len(repo.enqueued))
	}

	// Exhausted chains never run an upstream call.
	if err := w.HandleReportRetryOutbox(context.Background(), reportRetryJob(t, "report", 4)); err != nil {
		t.Fatal(err)
	}
	if failing.calls != 1 {
		t.Fatal("exhausted policy still called a provider")
	}
}

func TestReportRetrySkipsGoneConflictingAndHealthyRows(t *testing.T) {
	t.Parallel()
	c := DefaultConfig()
	for i := range c.Providers {
		c.Providers[i].Enabled = c.Providers[i].ID == "abuseipdb" || c.Providers[i].ID == "scamalytics"
	}
	healthy := partialReport(t, c)
	healthy.Status = "succeeded"

	gone := &reportRetryRepo{workerRepo: workerRepo{config: c}, gone: true}
	w := NewWorker(gone, nil)
	if err := w.HandleReportRetryOutbox(context.Background(), reportRetryJob(t, "report", 1)); err != nil {
		t.Fatal(err)
	}
	if gone.saves != 0 || len(gone.enqueued) != 0 {
		t.Fatal("gone row mutated")
	}

	terminal := &reportRetryRepo{workerRepo: workerRepo{config: c}}
	terminal.report = healthy
	w.repository = terminal
	if err := w.HandleReportRetryOutbox(context.Background(), reportRetryJob(t, "report", 1)); err != nil {
		t.Fatal(err)
	}
	if terminal.saves != 0 {
		t.Fatal("healthy row rewritten")
	}

	conflicting := &reportRetryRepo{workerRepo: workerRepo{config: c}}
	conflicting.report = partialReport(t, c)
	conflicting.conflict = true
	w.repository = conflicting
	if err := w.HandleReportRetryOutbox(context.Background(), reportRetryJob(t, "report", 1)); err != nil {
		t.Fatal(err)
	}
	if len(conflicting.enqueued) != 0 {
		t.Fatal("conflicting row chained")
	}
}
