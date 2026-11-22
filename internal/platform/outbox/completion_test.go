package outbox

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type completionRetryRepository struct {
	concurrentRepository
	attempts     atomic.Int32
	failures     int32
	committed    bool
	ambiguous    bool
	stored       chan model.OutboxJob
	storedError  chan error
	storeEffects atomic.Int32
}

func (r *completionRetryRepository) CompleteOutboxClaim(_ context.Context, job model.OutboxJob, outcome error, _ time.Time) error {
	attempt := r.attempts.Add(1)
	if !r.committed && (r.ambiguous || attempt > r.failures) {
		r.committed = true
		r.storeEffects.Add(1)
	}
	if attempt <= r.failures {
		return errors.New("completion connection interrupted")
	}
	r.stored <- job
	r.storedError <- outcome
	return nil
}

func TestCompletionRetriesPersistenceWithoutReplayingHandler(t *testing.T) {
	for _, ambiguous := range []bool{false, true} {
		name := "before_commit"
		if ambiguous {
			name = "after_commit"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			job := model.OutboxJob{ID: "receipt", Kind: "work", Attempts: 7, ClaimGeneration: 19}
			repo := &completionRetryRepository{
				concurrentRepository: concurrentRepository{jobs: []model.OutboxJob{job}},
				failures:             1, ambiguous: ambiguous, stored: make(chan model.OutboxJob, 1), storedError: make(chan error, 1),
			}
			worker := NewWorker(repo)
			var calls atomic.Int32
			outcome := &WaitError{Reason: "dependency pending", After: 2 * time.Second}
			if err := worker.Register("work", HandlerFunc(func(context.Context, model.OutboxJob) error {
				calls.Add(1)
				return outcome
			})); err != nil {
				t.Fatal(err)
			}
			if err := worker.Start(ctx, nil); err != nil {
				t.Fatal(err)
			}
			defer worker.Shutdown()
			if err := worker.Drain(ctx, 1); err != nil {
				t.Fatal(err)
			}
			select {
			case stored := <-repo.stored:
				if stored.ID != job.ID || stored.ClaimGeneration != job.ClaimGeneration || stored.Attempts != job.Attempts {
					t.Fatalf("completion lost claim fence: %+v", stored)
				}
			case <-ctx.Done():
				t.Fatal("completed work was left for lease recovery")
			}
			if stored := <-repo.storedError; !errors.Is(stored, outcome) {
				t.Fatalf("completion replaced handler outcome: %v", stored)
			}
			worker.Shutdown()
			if calls.Load() != 1 || repo.attempts.Load() != 2 || repo.storeEffects.Load() != 1 {
				t.Fatalf("handler=%d persistence=%d effects=%d", calls.Load(), repo.attempts.Load(), repo.storeEffects.Load())
			}
		})
	}
}

func TestCompletionPersistenceRetriesStayBounded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	repo := &completionRetryRepository{
		concurrentRepository: concurrentRepository{jobs: []model.OutboxJob{{ID: "failed", Kind: "work", Attempts: 1}}},
		failures:             100,
	}
	worker := NewWorker(repo)
	if err := worker.Register("work", HandlerFunc(func(context.Context, model.OutboxJob) error { return nil })); err != nil {
		t.Fatal(err)
	}
	reported := make(chan error, 1)
	if err := worker.Start(ctx, func(err error) { reported <- err }); err != nil {
		t.Fatal(err)
	}
	defer worker.Shutdown()
	if err := worker.Drain(ctx, 1); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-reported:
		if err == nil {
			t.Fatal("exhausted completion retries did not report their persistence error")
		}
	case <-ctx.Done():
		t.Fatal("persistence failure monopolized the executor")
	}
	worker.Shutdown()
	if repo.attempts.Load() != 3 || len(worker.executor.normal) != 0 {
		t.Fatal("completion retries did not release the lane within their bound")
	}
}
