package app

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/maintenance"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type maintenanceRecoveryRepository struct {
	maintenanceRepositoryFake
	claims int
}

func (r *maintenanceRecoveryRepository) ClaimMaintenanceRun(context.Context, string, string, time.Time, time.Time, bool) (string, bool, error) {
	r.claims++
	return "unexpected-rerun", true, nil
}

type maintenanceRecoveryBackup struct {
	calls int
}

func (b *maintenanceRecoveryBackup) Run(context.Context) (model.BackupRun, error) {
	b.calls++
	return model.BackupRun{ID: "unexpected-backup", Status: "complete"}, nil
}

func TestMaintenanceOperationRecoversTerminalItemWithoutAnotherRun(t *testing.T) {
	for _, status := range []providerops.Status{providerops.StatusSucceeded, providerops.StatusFailed} {
		t.Run(string(status), func(t *testing.T) {
			item := providerops.Item{
				OperationID: "operation-1", Key: "target", TargetType: "maintenance", TargetID: "retention",
				Status: status, ResultJSON: `{"backupRetentionWarning":true}`,
			}
			if status == providerops.StatusFailed {
				item.ErrorCode = "MAINTENANCE_FAILED"
			}
			repository := &maintenanceOperationRepositoryFake{items: []providerops.Item{item}}
			retention := &maintenanceRecoveryRepository{}
			backup := &maintenanceRecoveryBackup{}
			handler := &maintenanceOperationHandler{
				repository: repository, maintenance: maintenance.NewService(retention, backup, time.UTC), now: time.Now,
			}
			err := handler.HandleProviderOperation(context.Background(), providerops.Operation{Receipt: model.OperationReceipt{
				ID: "operation-1", Kind: providerops.KindAdminMaintenance, Status: string(providerops.StatusProcessing),
			}}, model.OutboxJob{ID: "job-1"})
			if err != nil {
				t.Fatal(err)
			}
			completion := repository.operationCompletion
			if completion.Status != status || completion.ResultJSON != item.ResultJSON || completion.ErrorCode != item.ErrorCode {
				t.Fatalf("durable maintenance outcome changed: %+v", completion)
			}
			if retention.claims != 0 || backup.calls != 0 || repository.itemCompletion.Status != "" {
				t.Fatalf("completed item repeated work: claims=%d backups=%d item=%+v", retention.claims, backup.calls, repository.itemCompletion)
			}
		})
	}
}
