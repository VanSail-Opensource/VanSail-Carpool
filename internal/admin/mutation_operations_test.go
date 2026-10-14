package admin

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestQueueMaintenanceIsIdempotentAndAudited(t *testing.T) {
	repository := &adminMutationRepository{
		adminCatalogRepository: &adminCatalogRepository{},
		operation: providerops.Operation{Receipt: model.OperationReceipt{
			ID: "operation-1", Kind: providerops.KindAdminMaintenance, Status: string(providerops.StatusQueued),
			CreatedAt: time.Date(2026, 8, 18, 2, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 8, 18, 2, 0, 0, 0, time.UTC),
		}},
	}
	service := newAdminServiceForTest(repository, nil, nil, nil, nil)
	first, err := service.QueueMaintenance(context.Background(), "admin-1", "maintenance-1")
	if err != nil {
		t.Fatalf("QueueMaintenance(first): %v", err)
	}
	repository.replay = true
	second, err := service.QueueMaintenance(context.Background(), "admin-1", "maintenance-1")
	if err != nil {
		t.Fatalf("QueueMaintenance(replay): %v", err)
	}
	if first.ID != second.ID || repository.creates != 1 {
		t.Fatalf("receipts=(%q,%q), creates=%d", first.ID, second.ID, repository.creates)
	}
	if len(repository.audits) != 1 || repository.audits[0].action != "admin_maintenance.queue" || repository.audits[0].targetType != "maintenance" {
		t.Fatalf("audits=%+v", repository.audits)
	}
	if len(repository.input.Items) != 1 || repository.input.Items[0].TargetID != "retention" {
		t.Fatalf("input=%+v", repository.input)
	}
}

type adminMutationRepository struct {
	*adminCatalogRepository
	operation providerops.Operation
	input     providerops.CreateInput
	replay    bool
	creates   int
}

func (r *adminMutationRepository) CreateProviderOperation(_ context.Context, input providerops.CreateInput, _ time.Time) (providerops.Operation, bool, error) {
	r.creates++
	r.input = input
	return r.operation, false, nil
}

func (r *adminMutationRepository) ProviderOperationForActorKey(_ context.Context, _ string, _ string, _ string, _ string) (model.OperationReceipt, bool, error) {
	return r.operation.Receipt, r.replay, nil
}

func (r *adminMutationRepository) ProviderOperationItems(context.Context, string) ([]providerops.Item, error) {
	return nil, nil
}

func (r *adminMutationRepository) BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error) {
	return r.operation, nil
}

func (r *adminMutationRepository) BeginProviderOperationItemAttempt(context.Context, string, string, time.Time) (providerops.Item, error) {
	return providerops.Item{}, nil
}

func (r *adminMutationRepository) CompleteProviderOperationItem(context.Context, string, string, providerops.Completion, time.Time) (providerops.Item, error) {
	return providerops.Item{}, nil
}

func (r *adminMutationRepository) CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error) {
	return r.operation, nil
}

func (r *adminMutationRepository) CompleteOutboxRetryOperation(context.Context, string, string, string, time.Time) error {
	return nil
}
