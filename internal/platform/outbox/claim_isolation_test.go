package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type blockedClaimRepository struct {
	concurrentRepository
	entered chan struct{}
	lateJob bool
}

func (r *blockedClaimRepository) ClaimOutboxJob(ctx context.Context, _ time.Time) (*model.OutboxJob, error) {
	close(r.entered)
	<-ctx.Done()
	if r.lateJob {
		return &model.OutboxJob{ID: "late", Kind: "work", Attempts: 1}, nil
	}
	return nil, ctx.Err()
}

func (r *blockedClaimRepository) ClaimPriorityOutboxJob(ctx context.Context, now time.Time) (*model.OutboxJob, error) {
	return r.concurrentRepository.ClaimOutboxJob(ctx, now)
}

func TestBlockedDatabaseClaimDoesNotBlockPriorityOrShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	repo := &blockedClaimRepository{
		concurrentRepository: concurrentRepository{
			jobs:      []model.OutboxJob{{ID: "urgent", Kind: "work", Attempts: 1}},
			completed: make(chan string, 2),
		},
		entered: make(chan struct{}),
	}
	w := NewWorker(repo)
	if err := w.Register("work", HandlerFunc(func(context.Context, model.OutboxJob) error { return nil })); err != nil {
		t.Fatal(err)
	}
	if err := w.Start(ctx, nil); err != nil {
		t.Fatal(err)
	}
	defer w.Shutdown()
	normal := make(chan error, 1)
	go func() { normal <- w.Drain(context.Background(), 1) }()
	select {
	case <-repo.entered:
	case <-ctx.Done():
		t.Fatal("ordinary claim did not start")
	}
	urgent := make(chan error, 1)
	go func() { urgent <- w.DrainPriority(ctx, 1) }()
	select {
	case err := <-urgent:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("ordinary database claim blocked urgent admission")
	}
	select {
	case id := <-repo.completed:
		if id != "urgent" {
			t.Fatal(id)
		}
	case <-ctx.Done():
		t.Fatal("urgent work did not complete")
	}
	stopped := make(chan struct{})
	go func() { w.Shutdown(); close(stopped) }()
	select {
	case <-stopped:
	case <-ctx.Done():
		t.Fatal("shutdown could not cancel the database claim")
	}
	if err := <-normal; !errors.Is(err, context.Canceled) {
		t.Fatalf("claim cancellation = %v", err)
	}
	if len(w.executor.normal) != 0 || len(w.executor.urgent) != 0 {
		t.Fatal("shutdown leaked an admission reservation")
	}
}

func TestClaimCompletingAfterShutdownDoesNotDispatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	repo := &blockedClaimRepository{
		concurrentRepository: concurrentRepository{completed: make(chan string, 1)},
		entered:              make(chan struct{}),
		lateJob:              true,
	}
	w := NewWorker(repo)
	called := make(chan struct{}, 1)
	if err := w.Register("work", HandlerFunc(func(context.Context, model.OutboxJob) error {
		called <- struct{}{}
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	if err := w.Start(ctx, nil); err != nil {
		t.Fatal(err)
	}
	drained := make(chan error, 1)
	go func() { drained <- w.Drain(context.Background(), 1) }()
	select {
	case <-repo.entered:
	case <-ctx.Done():
		t.Fatal("claim did not start")
	}
	stopped := make(chan struct{})
	go func() { w.Shutdown(); close(stopped) }()
	select {
	case <-stopped:
	case <-ctx.Done():
		t.Fatal("shutdown did not join the late claim")
	}
	if err := <-drained; err != nil {
		t.Fatal(err)
	}
	select {
	case <-called:
		t.Fatal("a claim returning during shutdown invoked its handler")
	default:
	}
	select {
	case id := <-repo.completed:
		if id != "late" {
			t.Fatal(id)
		}
	default:
		t.Fatal("late durable claim was not completed for recovery")
	}
}
