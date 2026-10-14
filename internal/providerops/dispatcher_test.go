package providerops

import (
	"context"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type dispatcherRepository struct{ operation Operation }

func (r dispatcherRepository) ProviderOperationByID(context.Context, string) (Operation, error) {
	return r.operation, nil
}

func TestDispatcherRoutesByOperationKind(t *testing.T) {
	called := false
	dispatcher := NewDispatcher(dispatcherRepository{operation: Operation{Receipt: model.OperationReceipt{ID: "operation", Kind: "reset", Status: string(StatusQueued)}}})
	if err := dispatcher.Register("reset", HandlerFunc(func(context.Context, Operation, model.OutboxJob) error {
		called = true
		return nil
	})); err != nil {
		t.Fatalf("Register(): %v", err)
	}
	job := model.OutboxJob{Kind: OutboxKind, Payload: `{"operationId":"operation"}`}
	if err := dispatcher.HandleOutbox(context.Background(), job); err != nil {
		t.Fatalf("HandleOutbox(): %v", err)
	}
	if !called {
		t.Fatal("registered handler was not called")
	}
}
