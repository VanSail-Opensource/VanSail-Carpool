package app

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/hourlybilling"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func newHourlyWorkflows(store *database.Store, policy hourlybilling.PolicySource, native *xnode.Service,
	vault *secret.Vault, dispatcher *providerops.Dispatcher, worker *outbox.Worker) (*hourlybilling.Service, error) {
	service := hourlybilling.New(store, policy, native, vault)
	for _, kind := range []string{hourlybilling.ActivateOperation, hourlybilling.DisableOperation} {
		if err := dispatcher.Register(kind, service); err != nil {
			return nil, err
		}
	}
	if err := worker.Register(hourlybilling.SettlementJob, service); err != nil {
		return nil, err
	}
	native.SetHourlyExtraBuyer(service)
	return service, nil
}

func (a *Application) enqueueHourly(ctx context.Context, now time.Time) {
	if a.hourly == nil {
		return
	}
	child, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := a.hourly.ScheduleDue(child, now.UTC()); err != nil && !errors.Is(err, context.Canceled) {
		a.logger.Error("hourly settlement admission failed", "error", err)
	}
}
