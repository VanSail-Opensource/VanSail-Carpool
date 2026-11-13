package iplookup

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type leaseWorkerRepository struct {
	workerRepo
	mu     sync.Mutex
	owners []string
}

func (r *leaseWorkerRepository) PendingIPLookupOperations(context.Context) ([]string, error) {
	return nil, nil
}

func (r *leaseWorkerRepository) ClaimIPLookupLease(_ context.Context, _, owner string, _ time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if owner == "" {
		return false, fmt.Errorf("empty lease owner")
	}
	for _, previous := range r.owners {
		if previous == owner {
			return false, fmt.Errorf("reused execution fence")
		}
	}
	r.owners = append(r.owners, owner)
	return true, nil
}

func (r *leaseWorkerRepository) requireLease(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.owners) == 0 || RunLeaseOwner(ctx) != r.owners[len(r.owners)-1] {
		return fmt.Errorf("repository context lost the current execution fence")
	}
	return nil
}

func (r *leaseWorkerRepository) HeartbeatIPLookup(ctx context.Context, _, owner string, _ time.Time) error {
	if owner != RunLeaseOwner(ctx) {
		return fmt.Errorf("heartbeat owner differs from execution fence")
	}
	return r.requireLease(ctx)
}

func (r *leaseWorkerRepository) ProviderOperationByID(ctx context.Context, id string) (providerops.Operation, error) {
	return providerops.Operation{Receipt: model.OperationReceipt{ID: id, Status: "processing"}}, r.requireLease(ctx)
}

func (r *leaseWorkerRepository) IPLookupRun(ctx context.Context, id string) (Report, Config, error) {
	if err := r.requireLease(ctx); err != nil {
		return Report{}, Config{}, err
	}
	return r.workerRepo.IPLookupRun(ctx, id)
}

func (r *leaseWorkerRepository) SaveIPLookupProgress(ctx context.Context, report Report) error {
	if err := r.requireLease(ctx); err != nil {
		return err
	}
	return r.workerRepo.SaveIPLookupProgress(ctx, report)
}

func (r *leaseWorkerRepository) FinishIPLookupRun(ctx context.Context, report Report, now time.Time) error {
	if err := r.requireLease(ctx); err != nil {
		return err
	}
	return r.workerRepo.FinishIPLookupRun(ctx, report, now)
}

type leaseCheckingProvider struct{ repository *leaseWorkerRepository }

func (p leaseCheckingProvider) ID() string { return "ipapi" }
func (p leaseCheckingProvider) Lookup(ctx context.Context, _ string, _ ProviderConfig) (ProviderResult, error) {
	// Live flights detach cancellation while retaining the typed ownership fence.
	return ProviderResult{Status: "success", Complete: true}, p.repository.requireLease(context.WithoutCancel(ctx))
}

func TestBackgroundUsesDistinctExecutionFencesAndPropagatesThem(t *testing.T) {
	t.Parallel()
	c := DefaultConfig()
	for i := range c.Providers {
		c.Providers[i].Enabled = c.Providers[i].ID == "ipapi"
	}
	repo := &leaseWorkerRepository{workerRepo: workerRepo{config: c}}
	w := NewWorker(repo, []Provider{leaseCheckingProvider{repo}})
	for range 2 {
		repo.report = NewReport("shared-report", "8.8.8.8", c)
		if err := w.executeBackground(context.Background(), repo, "operation"); err != nil {
			t.Fatal(err)
		}
	}
	if len(repo.owners) != 2 || repo.owners[0] == repo.owners[1] || repo.saves == 0 {
		t.Fatalf("missing execution fences or persisted stages: owners=%v saves=%d", repo.owners, repo.saves)
	}
}
