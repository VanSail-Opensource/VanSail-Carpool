package app

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

type isolatedAdmissionRepository struct {
	admissionRepository
	blockedPriority bool
	entered         chan struct{}
	released        chan struct{}
}

func (r *isolatedAdmissionRepository) claim(ctx context.Context, now time.Time, priority bool) (*model.OutboxJob, error) {
	if priority != r.blockedPriority {
		return r.admissionRepository.ClaimOutboxJob(ctx, now)
	}
	close(r.entered)
	<-ctx.Done()
	close(r.released)
	return nil, ctx.Err()
}

func (r *isolatedAdmissionRepository) ClaimOutboxJob(ctx context.Context, now time.Time) (*model.OutboxJob, error) {
	return r.claim(ctx, now, false)
}

func (r *isolatedAdmissionRepository) ClaimPriorityOutboxJob(ctx context.Context, now time.Time) (*model.OutboxJob, error) {
	return r.claim(ctx, now, true)
}

func TestAdmissionLanesProgressIndependentlyAndJoinShutdown(t *testing.T) {
	for _, priority := range []bool{false, true} {
		name := "ordinary_claim_blocked"
		if priority {
			name = "priority_claim_blocked"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			repo := &isolatedAdmissionRepository{blockedPriority: priority, entered: make(chan struct{}), released: make(chan struct{})}
			worker := outbox.NewWorker(repo)
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
			case <-repo.entered:
			case <-ctx.Done():
				t.Fatal("blocked lane never entered its database claim")
			}
			select {
			case <-delivered:
			case <-ctx.Done():
				t.Fatal("one database claim stalled the other admission lane")
			}
			cancel()
			<-done
			select {
			case <-repo.released:
			default:
				t.Fatal("admission shutdown left its database claim running")
			}
		})
	}
}
