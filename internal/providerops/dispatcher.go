package providerops

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

// OutboxKind is the single durable lane shared by all provider operations.
const OutboxKind = "provider_operation"

// OperationLoader loads the durable command selected by an outbox job.
type OperationLoader interface {
	ProviderOperationByID(context.Context, string) (Operation, error)
}

// Handler owns one provider operation kind.
type Handler interface {
	HandleProviderOperation(context.Context, Operation, model.OutboxJob) error
}

// HandlerFunc adapts a function to a provider operation Handler.
type HandlerFunc func(context.Context, Operation, model.OutboxJob) error

// HandleProviderOperation calls f.
func (f HandlerFunc) HandleProviderOperation(ctx context.Context, operation Operation, job model.OutboxJob) error {
	return f(ctx, operation, job)
}

// Dispatcher routes the shared provider-operation outbox lane by command kind.
type Dispatcher struct {
	repository OperationLoader
	mu         sync.RWMutex
	handlers   map[string]Handler
}

// NewDispatcher creates an empty operation-kind dispatcher.
func NewDispatcher(repository OperationLoader) *Dispatcher {
	return &Dispatcher{repository: repository, handlers: make(map[string]Handler)}
}

// Register assigns one unique operation kind to a handler.
func (d *Dispatcher) Register(kind string, handler Handler) error {
	if kind == "" || handler == nil {
		return errors.New("provider operation registration requires kind and handler")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.handlers[kind]; exists {
		return fmt.Errorf("register provider operation kind %q: duplicate registration", kind)
	}
	d.handlers[kind] = handler
	return nil
}

// Kinds returns registered operation kinds in stable order.
func (d *Dispatcher) Kinds() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	kinds := make([]string, 0, len(d.handlers))
	for kind := range d.handlers {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

// HandleOutbox dispatches one generic provider-operation job.
func (d *Dispatcher) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	operationID, err := jobpayload.TargetID(job, "operationId")
	if err != nil {
		return err
	}
	operation, err := d.repository.ProviderOperationByID(ctx, operationID)
	if err != nil {
		return fmt.Errorf("load provider operation: %w", err)
	}
	if Terminal(Status(operation.Receipt.Status)) {
		return nil
	}
	if gate, ok := d.repository.(interface {
		AccountOperationPermitted(context.Context, string) (bool, error)
	}); ok {
		allowed, err := gate.AccountOperationPermitted(ctx, operationID)
		if err != nil || !allowed {
			return err
		}
	}
	d.mu.RLock()
	handler := d.handlers[operation.Receipt.Kind]
	d.mu.RUnlock()
	if handler == nil {
		return fmt.Errorf("provider operation kind %q is not registered", operation.Receipt.Kind)
	}
	return handler.HandleProviderOperation(ctx, operation, job)
}
