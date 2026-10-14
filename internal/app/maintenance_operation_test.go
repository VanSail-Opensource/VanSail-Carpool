package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/maintenance"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestMaintenanceOperationCompletesSuccessAndFailureReceipts(t *testing.T) {
	tests := []struct {
		name         string
		backupStatus string
		backupErr    error
		wantStatus   providerops.Status
		wantCode     string
	}{
		{name: "success", backupStatus: "complete", wantStatus: providerops.StatusSucceeded},
		{name: "backup failure", backupErr: errors.New("disk full"), wantStatus: providerops.StatusFailed, wantCode: "MAINTENANCE_FAILED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &maintenanceOperationRepositoryFake{items: []providerops.Item{{
				OperationID: "operation-1", Key: "target", TargetType: "maintenance", TargetID: "retention", Status: providerops.StatusQueued,
			}}}
			maintenanceRepository := &maintenanceRepositoryFake{acquired: true}
			backup := &maintenanceBackupFake{run: model.BackupRun{ID: "backup-1", Status: test.backupStatus}, err: test.backupErr}
			service := maintenance.NewService(maintenanceRepository, backup, time.UTC)
			handler := &maintenanceOperationHandler{repository: repository, maintenance: service, databasePath: t.TempDir() + "\\database.db", now: func() time.Time {
				return time.Date(2026, 8, 18, 2, 0, 0, 0, time.UTC)
			}}
			err := handler.HandleProviderOperation(context.Background(), providerops.Operation{Receipt: model.OperationReceipt{
				ID: "operation-1", Kind: providerops.KindAdminMaintenance, Status: string(providerops.StatusQueued),
			}}, model.OutboxJob{ID: "job-1"})
			if err != nil {
				t.Fatalf("HandleProviderOperation(): %v", err)
			}
			if repository.operationCompletion.Status != test.wantStatus {
				t.Fatalf("operation status=%q, want %q", repository.operationCompletion.Status, test.wantStatus)
			}
			if repository.itemCompletion.Status != test.wantStatus {
				t.Fatalf("item status=%q, want %q", repository.itemCompletion.Status, test.wantStatus)
			}
			if repository.operationCompletion.ErrorCode != test.wantCode {
				t.Fatalf("operation error code=%q, want %q", repository.operationCompletion.ErrorCode, test.wantCode)
			}
		})
	}
}

func TestMaintenanceOperationRetriesWhenActiveLeaseIsHeld(t *testing.T) {
	repository := &maintenanceOperationRepositoryFake{items: []providerops.Item{{
		OperationID: "operation-1", Key: "target", TargetType: "maintenance", TargetID: "retention", Status: providerops.StatusQueued,
	}}}
	service := maintenance.NewService(&maintenanceRepositoryFake{acquired: false}, &maintenanceBackupFake{}, time.UTC)
	handler := &maintenanceOperationHandler{repository: repository, maintenance: service, databasePath: t.TempDir() + "\\database.db", now: time.Now}
	if err := handler.HandleProviderOperation(context.Background(), providerops.Operation{Receipt: model.OperationReceipt{
		ID: "operation-1", Kind: providerops.KindAdminMaintenance, Status: string(providerops.StatusQueued),
	}}, model.OutboxJob{ID: "job-1"}); !errors.Is(err, maintenance.ErrBusy) {
		t.Fatalf("HandleProviderOperation() error=%v, want ErrBusy", err)
	}
	if repository.operationCompletion.Status != "" {
		t.Fatalf("busy operation was completed: %+v", repository.operationCompletion)
	}
}

type maintenanceOperationRepositoryFake struct {
	items               []providerops.Item
	operationCompletion providerops.Completion
	itemCompletion      providerops.Completion
}

func (r *maintenanceOperationRepositoryFake) ProviderOperationItems(context.Context, string) ([]providerops.Item, error) {
	return r.items, nil
}
func (r *maintenanceOperationRepositoryFake) BeginProviderOperationAttempt(_ context.Context, _ string, _ time.Time) (providerops.Operation, error) {
	return providerops.Operation{Receipt: model.OperationReceipt{ID: "operation-1", Status: string(providerops.StatusProcessing)}}, nil
}
func (r *maintenanceOperationRepositoryFake) BeginProviderOperationItemAttempt(_ context.Context, _ string, _ string, _ time.Time) (providerops.Item, error) {
	r.items[0].Status = providerops.StatusProcessing
	return r.items[0], nil
}
func (r *maintenanceOperationRepositoryFake) CompleteProviderOperationItem(_ context.Context, _ string, _ string, completion providerops.Completion, _ time.Time) (providerops.Item, error) {
	r.itemCompletion = completion
	return r.items[0], nil
}
func (r *maintenanceOperationRepositoryFake) CompleteProviderOperation(_ context.Context, _ string, completion providerops.Completion, _ time.Time) (providerops.Operation, error) {
	r.operationCompletion = completion
	return providerops.Operation{}, nil
}

type maintenanceRepositoryFake struct {
	acquired bool
}

func (r *maintenanceRepositoryFake) ClaimMaintenanceRun(context.Context, string, string, time.Time, time.Time, bool) (string, bool, error) {
	return "maintenance-run-1", r.acquired, nil
}
func (r *maintenanceRepositoryFake) CompactAndPrune(context.Context, time.Time, time.Time, time.Time) (map[string]int64, error) {
	return map[string]int64{"sessions": 1}, nil
}
func (r *maintenanceRepositoryFake) CompleteMaintenanceRun(context.Context, string, string, map[string]int64, error, time.Time) error {
	return nil
}

type maintenanceBackupFake struct {
	run model.BackupRun
	err error
}

func (b *maintenanceBackupFake) Run(context.Context) (model.BackupRun, error) {
	return b.run, b.err
}
