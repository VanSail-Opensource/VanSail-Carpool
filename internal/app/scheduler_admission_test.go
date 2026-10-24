package app

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

type admissionRepository struct {
	mu      sync.Mutex
	claimed bool
}

func (r *admissionRepository) ClaimOutboxJob(context.Context, time.Time) (*model.OutboxJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.claimed {
		return nil, nil
	}
	r.claimed = true
	return &model.OutboxJob{ID: "member", Kind: "member", Attempts: 1}, nil
}

func (*admissionRepository) ClaimPriorityOutboxJob(context.Context, time.Time) (*model.OutboxJob, error) {
	return nil, nil
}

func (*admissionRepository) CompleteOutboxJob(context.Context, string, int, error, time.Time) error {
	return nil
}

func (*admissionRepository) RenewOutboxJob(context.Context, string, int, time.Time) (bool, error) {
	return true, nil
}

func TestAdmissionPumpDispatchesMembersWithoutScheduledDrain(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	worker := outbox.NewWorker(&admissionRepository{})
	delivered := make(chan struct{})
	if err := worker.Register("member", outbox.HandlerFunc(func(context.Context, model.OutboxJob) error {
		close(delivered)
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	if err := worker.Start(ctx, nil); err != nil {
		t.Fatal(err)
	}
	defer worker.Shutdown()
	a := &Application{outbox: worker, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	done := make(chan struct{})
	go func() { defer close(done); a.runOutboxAdmission(ctx) }()
	defer func() { cancel(); <-done }()
	select {
	case <-delivered:
	case <-ctx.Done():
		t.Fatal("member admission depended on the scheduler reaching its normal drain")
	}
}
