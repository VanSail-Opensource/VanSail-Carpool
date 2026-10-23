package outbox

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"sync"
	"testing"
	"time"
)

type concurrentRepository struct {
	mu        sync.Mutex
	jobs      []model.OutboxJob
	completed chan string
}

func (r *concurrentRepository) ClaimOutboxJob(context.Context, time.Time) (*model.OutboxJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.jobs) == 0 {
		return nil, nil
	}
	job := r.jobs[0]
	r.jobs = r.jobs[1:]
	return &job, nil
}
func (r *concurrentRepository) CompleteOutboxJob(_ context.Context, id string, _ int, _ error, _ time.Time) error {
	r.completed <- id
	return nil
}
func (r *concurrentRepository) RenewOutboxJob(context.Context, string, int, time.Time) (bool, error) {
	return true, nil
}

func TestExecutorAdmitsIndependentWorkAndJoinsCancellation(t *testing.T) {
	repo := &concurrentRepository{jobs: []model.OutboxJob{{ID: "slow", Kind: "work", Attempts: 1}, {ID: "fast", Kind: "work", Attempts: 1}}, completed: make(chan string, 2)}
	w := NewWorker(repo)
	started := make(chan string, 2)
	if err := w.Register("work", HandlerFunc(func(ctx context.Context, j model.OutboxJob) error {
		started <- j.ID
		if j.ID == "slow" {
			<-ctx.Done()
			return ctx.Err()
		}
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := w.Start(ctx, nil); err != nil {
		t.Fatal(err)
	}
	defer w.Shutdown()
	if err := w.Drain(ctx, 10); err != nil {
		t.Fatal(err)
	}
	select {
	case id := <-repo.completed:
		if id != "fast" {
			t.Fatalf("slow attempt blocked independent work: %s", id)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	w.Shutdown()
	select {
	case id := <-repo.completed:
		if id != "slow" {
			t.Fatal(id)
		}
	default:
		t.Fatal("shutdown did not join the active attempt")
	}
}
