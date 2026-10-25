package entitlements

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func (w *Worker) prepareContinuity(ctx context.Context, job model.OutboxJob) error {
	successorID, err := jobpayload.TargetID(job, "purchaseId")
	if err != nil {
		return err
	}
	// Upgrade compatibility only: native compilation already prepares paid
	// successors. Replaying a retired job must never mutate a period or account.
	_ = successorID
	return nil
}
