package iplookup

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type repairCallbackProvider struct {
	run func()
}

func (repairCallbackProvider) ID() string { return "scamalytics" }

func (p repairCallbackProvider) Lookup(context.Context, string, ProviderConfig) (ProviderResult, error) {
	p.run()
	return ProviderResult{ID: "scamalytics", Status: "success", Complete: true, Facts: Facts{Country: "JP"}}, nil
}

type repairCallbackResolver struct {
	run func()
}

func (r repairCallbackResolver) LookupAddr(context.Context, string) ([]string, error) {
	r.run()
	return []string{"older.example."}, nil
}

func TestReportRepairLosingRevisionRetainsNewerEvidence(t *testing.T) {
	t.Parallel()
	c := DefaultConfig()
	for i := range c.Providers {
		c.Providers[i].Enabled = c.Providers[i].ID == "abuseipdb" || c.Providers[i].ID == "scamalytics"
	}
	repo := &reportRetryRepo{workerRepo: workerRepo{config: c}}
	repo.report = partialReport(t, c)
	provider := repairCallbackProvider{run: func() {
		// Another replica commits while this request is outside the database.
		repo.report.RepairRevision = "newer-committed-revision"
		repo.report.Facts.Country = "CA"
	}}
	worker := NewWorker(repo, []Provider{provider})
	if err := worker.HandleReportRetryOutbox(context.Background(), reportRetryJob(t, "report", 1)); err != nil {
		t.Fatal(err)
	}
	if repo.report.Facts.Country != "CA" || repo.report.RepairRevision != "newer-committed-revision" || len(repo.enqueued) != 0 {
		t.Fatal("losing repair overwrote or chained newer evidence", repo.report, repo.enqueued)
	}
}

func TestLiveRepairLosingRevisionRetainsNewerEvidence(t *testing.T) {
	t.Parallel()
	repo := &liveRetryRepo{details: partiallyFailingPTR(), status: "partial"}
	resolver := repairCallbackResolver{run: func() {
		repo.details.RepairRevision = "newer-committed-revision"
		repo.details.PTR = PTRDetails{LiveMeta: healthyMeta(), Domains: []string{"newer.example."}}
	}}
	service := retryLiveService(t, resolver, repo)
	if err := service.HandleLiveRetryOutbox(context.Background(), liveRetryJob("snapshot", 1)); err != nil {
		t.Fatal(err)
	}
	if repo.updates != 0 || len(repo.details.PTR.Domains) != 1 || repo.details.PTR.Domains[0] != "newer.example." || repo.details.RepairRevision != "newer-committed-revision" || len(repo.enqueued) != 0 {
		t.Fatal("losing live repair overwrote or chained newer evidence", repo.details, repo.enqueued)
	}
}

func TestRepairRevisionsStayOutsidePublicAndStoredDocuments(t *testing.T) {
	t.Parallel()
	const token = "private-repair-revision"
	report := Report{RepairRevision: token, IP: "1.1.1.1", Status: "partial"}
	details := partiallyFailingPTR()
	details.RepairRevision = token
	for _, value := range []any{PublicReport(report), details} {
		raw, err := json.Marshal(value)
		if err != nil || strings.Contains(string(raw), token) || strings.Contains(string(raw), "RepairRevision") {
			t.Fatal("internal repair revision leaked", string(raw), err)
		}
	}
	raw, err := EncodeReport(report)
	if err != nil || strings.Contains(string(raw), token) {
		t.Fatal("stored report duplicated its revision", string(raw), err)
	}
}
