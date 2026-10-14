package iplookup

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestWorkerRPKISettlesAfterReputationAndNeverReplaysCompletedEvidence(t *testing.T) {
	c := DefaultConfig()
	c.Providers[0].Enabled = true
	repo := &workerRepo{config: c, report: NewReport("r", "1.1.1.1", c)}
	provider := &fakeProvider{id: "abuseipdb", result: ProviderResult{Status: "success", Complete: true, Facts: Facts{NetworkType: "residential"}, Signals: Signals{Abuse: boolean(false), Datacenter: boolean(false), VPN: boolean(false), Proxy: boolean(false), Tor: boolean(false)}}}
	var calls atomic.Int32
	s := liveTestService(t, "ripe", func(r *http.Request) (int, string) {
		calls.Add(1)
		if provider.calls != 1 || repo.report.Checkpoint.RPKI.Status != "processing" || !repo.report.Checkpoint.RPKI.Attempted {
			t.Error("RPKI request preceded durable stage or reputation work")
		}
		if strings.Contains(r.URL.Path, "network-info") {
			return 200, `{"data":{"prefix":"1.1.1.0/24","asns":[13335]}}`
		}
		return 200, `{"data":{"resource":"13335","prefix":"1.1.1.0/24","status":"invalid_length"}}`
	})
	w := NewWorker(repo, []Provider{provider})
	w.Live = s
	if err := w.HandleProviderOperation(context.Background(), providerops.Operation{}, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	if repo.report.Verdict != "unsuitable" || len(repo.report.Databases) != 1 || repo.report.RefundRequired || repo.report.Checkpoint.RPKI.Status != "success" {
		t.Fatalf("bad final report: %+v", repo.report)
	}
	if err := w.HandleProviderOperation(context.Background(), providerops.Operation{}, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 || calls.Load() != 2 {
		t.Fatal("completed durable work replayed")
	}
}

func TestInterruptedRPKIStageDoesNotReplayAndRemainsNeutral(t *testing.T) {
	repo := &workerRepo{report: NewReport("r", "1.1.1.1", DefaultConfig())}
	repo.report.Checkpoint.RPKI.Status = "processing"
	var calls atomic.Int32
	s := liveTestService(t, "ripe", func(*http.Request) (int, string) { calls.Add(1); return 200, `{}` })
	w := NewWorker(repo, nil)
	w.Live = s
	if err := w.runRPKI(context.Background(), &repo.report, func(r Report) error { repo.report = r; return nil }); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || len(repo.report.Refusals) != 0 || repo.report.Checkpoint.RPKI.Status != "error" || ReportAttempted(repo.report) {
		t.Fatal("interruption replayed, refused or changed provider billing marker")
	}
}

func TestInterruptedRPKIRepairKeepsVerifiedInvalidEvidence(t *testing.T) {
	repo := &workerRepo{report: NewReport("r", "1.1.1.1", DefaultConfig())}
	repo.report.Checkpoint.RPKI.Status = "processing"
	repo.report.RPKI = &RPKIResult{Prefix: "1.1.1.0/24", LiveMeta: LiveMeta{Status: "partial"}, Routes: []RPKIRoute{{ASN: 13335, Validity: "invalid_asn", LiveMeta: LiveMeta{Status: "success"}}}}
	w := NewWorker(repo, nil)
	if err := w.runRPKI(context.Background(), &repo.report, func(r Report) error { repo.report = r; return nil }); err != nil {
		t.Fatal(err)
	}
	if len(repo.report.Refusals) != 1 || repo.report.Refusals[0].Kind != "rpki_invalid_asn" {
		t.Fatal("interrupted repair erased confirmed invalid route")
	}
}
