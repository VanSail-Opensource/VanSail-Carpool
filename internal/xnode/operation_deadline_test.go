package xnode

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type operationDeadlineRepository struct {
	Repository
	deadlines chan time.Time
}

func (r *operationDeadlineRepository) RenewOutboxClaim(ctx context.Context, _ model.OutboxJob, _ time.Time) (bool, error) {
	deadline, _ := ctx.Deadline()
	select {
	case r.deadlines <- deadline:
	default:
	}
	return true, nil
}

func (*operationDeadlineRepository) CompleteOutboxClaim(context.Context, model.OutboxJob, error, time.Time) error {
	return nil
}

func TestProviderOperationLeaseCoversNativeUpdateExecution(t *testing.T) {
	repo := &operationDeadlineRepository{deadlines: make(chan time.Time, 1)}
	service := NewService(repo, nil, nil, nil, nil)
	master := NewMaster(service, nil)
	ctx, cancel := context.WithCancel(context.Background())
	master.ctx = ctx
	t.Cleanup(func() { cancel(); master.Wait() })
	started := time.Now()
	err := service.HandleProviderOperation(ctx, providerops.Operation{Receipt: model.OperationReceipt{Kind: OperationKind}}, model.OutboxJob{Kind: "provider_operation"})
	if !errors.Is(err, outbox.ErrDeferred) {
		t.Fatal("native command did not enter the durable deferred executor", err)
	}
	select {
	case deadline := <-repo.deadlines:
		for _, command := range []string{"update.install", "update.rollback", "certificates.run"} {
			// Preflight may consume 30 seconds before the command's separate
			// 30-second delivery and full execution window even begins.
			minimum := started.Add(time.Minute + commandDeadline(command))
			if !deadline.After(minimum) {
				t.Fatalf("outer lease deadline %v truncates %s completion window ending %v", deadline, command, minimum)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("deferred command did not acquire its durable claim")
	}
}
