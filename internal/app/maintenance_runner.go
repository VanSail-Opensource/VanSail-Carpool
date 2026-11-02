package app

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/maintenance"
)

type maintenanceTaskResult struct {
	maintenance.RunResult
}

// runMaintenanceTask keeps the scheduled and manual commands on the same
// backup-gated PostgreSQL cleanup path.
func runMaintenanceTask(ctx context.Context, service *maintenance.Service, at time.Time,
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

	return result, nil
}
