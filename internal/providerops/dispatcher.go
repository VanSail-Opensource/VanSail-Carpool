package providerops

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
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
	status := Status(operation.Receipt.Status)
	if status == StatusSucceeded || status == StatusFailed || status == StatusCompensated {
		return nil
	}
	if gate, ok := d.repository.(interface {
		AccountOperationPermitted(context.Context, string) (bool, error)
	}); ok {
		allowed, err := gate.AccountOperationPermitted(ctx, operationID)
		if err != nil {
			return err
		}
		if !allowed {
			// A lifecycle fence forbids new mutations, but cannot prove that an
			// earlier external attempt failed. Keep its durable continuation.
			return &outbox.WaitError{Reason: "account_operation_reconciliation_pending", After: time.Minute}
		}
	}
	if Terminal(status) {
		return providerContinuation(status)
	}
	d.mu.RLock()
	handler := d.handlers[operation.Receipt.Kind]
	d.mu.RUnlock()
	if handler == nil {
		return fmt.Errorf("provider operation kind %q is not registered", operation.Receipt.Kind)
	}
	if err := handler.HandleProviderOperation(ctx, operation, job); err != nil {
		return err
	}
	// A nil handler result is not evidence of a completed external command.
	// Retain the encrypted target until durable receipt state confirms it.
	operation, err = d.repository.ProviderOperationByID(ctx, operationID)
	if err != nil {
		return fmt.Errorf("reload provider operation completion: %w", err)
	}
	return providerContinuation(Status(operation.Receipt.Status))
}

func providerContinuation(status Status) error {
	switch status {
	case StatusSucceeded, StatusFailed, StatusCompensated:
		return nil
	case StatusPendingReview, StatusPartial:
		return &outbox.ReviewError{Reason: "provider_operation_reconciliation_required"}
	default:
		return &outbox.WaitError{Reason: "provider_operation_completion_pending", After: time.Minute}
	}
}
