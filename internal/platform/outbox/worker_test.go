package outbox

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestWorkerDispatchesRegisteredKinds(t *testing.T) {
	t.Parallel()
	repository := &workerRepository{jobs: []*model.OutboxJob{
		{ID: "1", Kind: "alpha", Attempts: 1},
		{ID: "2", Kind: "beta", Attempts: 1},
	}}
	worker := NewWorker(repository)
	worker.now = func() time.Time { return time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC) }
	called := make([]string, 0, 2)
	if err := worker.Register("beta", HandlerFunc(func(_ context.Context, job model.OutboxJob) error {
		called = append(called, job.Kind)
		return nil
	})); err != nil {
		t.Fatalf("Register(beta): %v", err)
	}
	if err := worker.Register("alpha", HandlerFunc(func(_ context.Context, job model.OutboxJob) error {
		called = append(called, job.Kind)
		return nil
	})); err != nil {
		t.Fatalf("Register(alpha): %v", err)
	}

	if err := worker.Drain(context.Background(), 3); err != nil {
		t.Fatalf("Drain(): %v", err)
	}
	if !reflect.DeepEqual(called, []string{"alpha", "beta"}) {
		t.Fatalf("called = %v", called)
	}
	if !reflect.DeepEqual(worker.Kinds(), []string{"alpha", "beta"}) {
		t.Fatalf("Kinds() = %v", worker.Kinds())
	}
	if len(repository.completions) != 2 || repository.completions[0] != nil || repository.completions[1] != nil {
		t.Fatalf("completions = %v", repository.completions)
	}
}

func TestWorkerPersistsHandlerAndUnknownKindErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		kind    string
		handler Handler
		want    error
	}{
		{name: "handler", kind: "known", handler: HandlerFunc(func(context.Context, model.OutboxJob) error { return errors.New("provider unavailable") })},
		{name: "unknown", kind: "unknown", want: ErrUnknownKind},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &workerRepository{jobs: []*model.OutboxJob{{ID: "job", Kind: test.kind, Attempts: 3}}}
			worker := NewWorker(repository)
			if test.handler != nil {
				if err := worker.Register(test.kind, test.handler); err != nil {
					t.Fatalf("Register(): %v", err)
				}
			}
			if err := worker.Drain(context.Background(), 1); err != nil {
				t.Fatalf("Drain(): %v", err)
			}
			if len(repository.completions) != 1 || repository.completions[0] == nil {
				t.Fatalf("completion = %v", repository.completions)
			}
			if test.want != nil && !errors.Is(repository.completions[0], test.want) {
				t.Fatalf("completion error = %v, want %v", repository.completions[0], test.want)
			}
		})
	}
}

func TestWorkerRegistrationValidation(t *testing.T) {
	t.Parallel()
	worker := NewWorker(&workerRepository{})
	if err := worker.Register("", HandlerFunc(func(context.Context, model.OutboxJob) error { return nil })); err == nil {
		t.Fatal("empty kind accepted")
	}
	if err := worker.Register("alpha", HandlerFunc(func(context.Context, model.OutboxJob) error { return nil })); err != nil {
		t.Fatalf("Register(): %v", err)
	}
	if err := worker.Register("alpha", HandlerFunc(func(context.Context, model.OutboxJob) error { return nil })); err == nil {
		t.Fatal("duplicate kind accepted")
	}
}

type workerRepository struct {
	jobs        []*model.OutboxJob
	next        int
	completions []error
}

func (r *workerRepository) ClaimOutboxJob(context.Context, time.Time) (*model.OutboxJob, error) {
	if r.next >= len(r.jobs) {
		return nil, nil
	}
	job := r.jobs[r.next]
	r.next++
	return job, nil
}

func (r *workerRepository) CompleteOutboxJob(_ context.Context, _ string, _ int, jobErr error, _ time.Time) error {
	r.completions = append(r.completions, jobErr)
	return nil
}
