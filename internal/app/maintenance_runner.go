package app

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/maintenance"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type maintenanceTaskResult struct {
	maintenance.RunResult
	MigrationSnapshotRetentionWarning error
}

// runMaintenanceTask keeps the scheduled and manual commands on the same
// backup-gated cleanup path, including adjacent migration snapshot pruning.
func runMaintenanceTask(ctx context.Context, service *maintenance.Service, databasePath string, at time.Time,
	force bool) (maintenanceTaskResult, error) {
	var (
		result maintenanceTaskResult
		err    error
	)
	if force {
		result.RunResult, err = service.RunManual(ctx, at)
	} else {
		result.RunResult, err = service.Run(ctx, at)
	}
	if err != nil {
		return result, err
	}
	result.MigrationSnapshotRetentionWarning = database.PruneExpansionBackups(databasePath, at)
	return result, nil
}
