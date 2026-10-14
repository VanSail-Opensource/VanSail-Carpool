package app

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

func registerAdminUserOutboxHandlers(worker *outbox.Worker, store *database.Store) error {
	return worker.Register(database.AdminTemporaryBanExpiryOutboxKind, outbox.HandlerFunc(func(ctx context.Context, job model.OutboxJob) error {
		userID, err := jobpayload.TargetID(job, "userId")
		if err != nil {
			return err
		}
		return store.QueueExpiredAdminTemporaryBan(ctx, userID, job.AvailableAt.UTC())
	}))
}
