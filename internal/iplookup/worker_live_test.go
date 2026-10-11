package iplookup

import (
	"context"
	"testing"

	"github.com/txyyddss/Remna-User-Panel/internal/model"
	"github.com/txyyddss/Remna-User-Panel/internal/providerops"
)

type workerLiveRepo struct {
	workerRepo
	stage LiveStage
}

func (r *workerLiveRepo) ClaimIPLookupLiveStage(context.Context, string) (LiveStage, bool, error) {
	claim := r.stage.Status == "queued"
	if claim {
		r.stage.Status = "processing"
	}
	return r.stage, claim, nil
}
func (r *workerLiveRepo) MarkIPLookupLiveAttempt(context.Context, string) error {
	r.stage.Attempted = true
	return nil
}
func (r *workerLiveRepo) FinishIPLookupLiveStage(_ context.Context, _ string, stage LiveStage) error {
	stage.Attempted = r.stage.Attempted
	r.stage = stage
	return nil
}

func TestCombinedWorkerRefreshesLiveAfterRefusalAndDoesNotReplay(t *testing.T) {
	s, _, hosts := integratedLiveService(t, false)
	s.repository = &snapshotMemory{}
	c := DefaultConfig()
	c.Providers[0].Enabled = true
	repo := &workerLiveRepo{workerRepo: workerRepo{config: c, report: NewReport("combined", "1.1.1.1", c)}, stage: LiveStage{Status: "queued"}}
	provider := &fakeProvider{id: "abuseipdb", result: ProviderResult{Status: "success", Complete: true, Signals: Signals{VPN: boolean(true)}}}
	w := NewWorker(repo, []Provider{provider})
	w.Live = s
	if err := w.HandleProviderOperation(context.Background(), providerops.Operation{}, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 || hosts.Load() != 1 || repo.report.Verdict != "unsuitable" || !repo.stage.Attempted || repo.stage.SnapshotID == "" {
		t.Fatalf("refusal lost live phase: %+v", repo.stage)
	}
	if err := w.HandleProviderOperation(context.Background(), providerops.Operation{}, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 || hosts.Load() != 1 {
		t.Fatal("completed stages replayed")
	}
}
