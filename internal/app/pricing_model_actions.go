package app

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

func registerPricingModelActions(worker *outbox.Worker, store *database.Store) error {
	if err := worker.Register("pricing_model_action", outbox.HandlerFunc(func(ctx context.Context, job model.OutboxJob) error {
		id, err := jobpayload.TargetID(job, "interventionId")
		if err != nil {
			return err
		}
		work, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		return store.ExecutePricingModelAction(work, id, time.Now().UTC())
	})); err != nil {
		return err
	}

	return nil
}
