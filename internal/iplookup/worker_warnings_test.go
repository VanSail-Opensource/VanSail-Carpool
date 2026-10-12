package iplookup

import (
	"context"
	"github.com/txyyddss/Remna-User-Panel/internal/model"
	"github.com/txyyddss/Remna-User-Panel/internal/providerops"
	"testing"
)

func TestDatacenterWarningContinuesProviderStages(t *testing.T) {
	c := DefaultConfig()
	c.Providers[0].Enabled = true
	c.Providers[1].Enabled = true
	repo := &workerRepo{config: c, report: NewReport("report", "8.8.8.8", c)}
	first := &fakeProvider{id: "abuseipdb", result: ProviderResult{ID: "abuseipdb", Status: "success", Complete: true, Signals: Signals{Datacenter: boolean(true)}}}
	next := &fakeProvider{id: "scamalytics", result: ProviderResult{ID: "scamalytics", Status: "success", Complete: true}}
	worker := NewWorker(repo, []Provider{first, next})
	if err := worker.HandleProviderOperation(context.Background(), providerops.Operation{}, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	if first.calls != 1 || next.calls != 1 || len(repo.report.Warnings) != 1 || len(repo.report.Refusals) != 0 || repo.report.PolicyVersion != RatingPolicyVersion {
		t.Fatalf("datacenter stopped the provider chain: %+v", repo.report)
	}
}
