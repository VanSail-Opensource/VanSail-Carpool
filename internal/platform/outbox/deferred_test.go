package outbox

import (
	"context"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"testing"
)

func TestLongNodeCommandKeepsItsLeaseWhilePurchaseDispatchContinues(t *testing.T) {
	repo := &workerRepository{jobs: []*model.OutboxJob{{ID: "command", Kind: "node", Attempts: 1}, {ID: "purchase", Kind: "paid", Attempts: 1}}}
	worker := NewWorker(repo)
	if err := worker.Register("node", HandlerFunc(func(context.Context, model.OutboxJob) error { return fmt.Errorf("awaiting agent: %w", ErrDeferred) })); err != nil {
		t.Fatal(err)
	}
	paid := false
	if err := worker.Register("paid", HandlerFunc(func(context.Context, model.OutboxJob) error { paid = true; return nil })); err != nil {
		t.Fatal(err)
	}
	if err := worker.Drain(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
	if !paid || len(repo.completions) != 1 || repo.completions[0] != nil {
		t.Fatal("command wait blocked provisioning or prematurely completed its durable lease")
	}
}
