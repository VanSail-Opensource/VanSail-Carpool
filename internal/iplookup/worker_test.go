package iplookup

import (
	"context"
	"testing"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/model"
	"github.com/txyyddss/Remna-User-Panel/internal/providerops"
)

type workerRepo struct {
	report Report
	config Config
	saves  int
}

func (r *workerRepo) BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error) {
	return providerops.Operation{}, nil
}

func (r *workerRepo) IPLookupRun(context.Context, string) (Report, Config, error) {
	return r.report, r.config, nil
}
func (r *workerRepo) SaveIPLookupProgress(_ context.Context, report Report) error {
	r.report = report
	r.saves++
	return nil
}
func (r *workerRepo) FinishIPLookupRun(_ context.Context, report Report, _ time.Time) error {
	r.report = report
	return nil
}

type fakeProvider struct {
	id     string
	calls  int
	result ProviderResult
	err    error
}

func (p *fakeProvider) ID() string { return p.id }
func (p *fakeProvider) Lookup(context.Context, string, ProviderConfig) (ProviderResult, error) {
	p.calls++
	return p.result, p.err
}

func TestWorkerStopsAndDoesNotReplayCompletedStages(t *testing.T) {
	t.Parallel()
	c := DefaultConfig()
	for i := range c.Providers {
		c.Providers[i].Enabled = true
	}
	r := &workerRepo{config: c, report: NewReport("report", "150.249.241.62", c)}
	RecordProvider(&r.report, ProviderResult{ID: "abuseipdb", Status: "success", Complete: true}, c)
	first := &fakeProvider{id: "abuseipdb"}
	second := &fakeProvider{id: "scamalytics", result: ProviderResult{ID: "scamalytics", Status: "partial", Signals: Signals{VPN: boolean(true)}}}
	later := &fakeProvider{id: "ipapi"}
	w := NewWorker(r, []Provider{first, second, later})
	if err := w.HandleProviderOperation(context.Background(), providerops.Operation{}, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	// Terminal reports retain their in-memory checkpoint; the codec strips it
	// on encode (covered by TestCompactCheckpointAndLegacyOutcomePreservation).
	if first.calls != 0 || second.calls != 1 || later.calls != 0 || r.report.Verdict != "unsuitable" || len(r.report.Databases) != 2 {
		t.Fatalf("replayed/stopped incorrectly: %+v", r.report)
	}
	if err := w.HandleProviderOperation(context.Background(), providerops.Operation{}, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	if second.calls != 1 {
		t.Fatal("terminal report made another upstream request")
	}
}

func TestWorkerContinuesErrorsAndMarksInterruptedStage(t *testing.T) {
	t.Parallel()
	c := DefaultConfig()
	c.Providers[0].Enabled = true
	c.Providers[1].Enabled = true
	r := &workerRepo{config: c, report: NewReport("report", "150.249.241.62", c)}
	r.report.Checkpoint.Stages[0].Status = "processing"
	r.report.Checkpoint.Stages[0].Attempted = true
	first := &fakeProvider{id: "abuseipdb"}
	second := &fakeProvider{id: "scamalytics", result: ProviderResult{ID: "scamalytics", Status: "success", Complete: true, Facts: Facts{Country: "JP"}}}
	if err := NewWorker(r, []Provider{first, second}).HandleProviderOperation(context.Background(), providerops.Operation{}, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	if first.calls != 0 || second.calls != 1 || r.report.Status != "partial" || r.report.RefundRequired || r.report.Databases[0].Status != "error" {
		t.Fatalf("report=%+v", r.report)
	}
}
