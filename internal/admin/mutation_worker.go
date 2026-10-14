package admin

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// MutationWorker owns generic retry and provider-aware payment refund commands.
type MutationWorker struct {
	service    *Service
	repository mutationOperationRepository
	now        func() time.Time
}

// NewMutationWorker creates the remaining administrator operation handler.
func NewMutationWorker(service *Service) (*MutationWorker, error) {
	if service == nil {
		return nil, errors.New("administrator mutation service is unavailable")
	}
	repository, ok := service.repository.(mutationOperationRepository)
	if !ok {
		return nil, errors.New("administrator mutation repository is unavailable")
	}
	return &MutationWorker{service: service, repository: repository, now: time.Now}, nil
}

// HandleProviderOperation dispatches one administrator mutation kind.
func (w *MutationWorker) HandleProviderOperation(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	switch operation.Receipt.Kind {
	case providerops.KindOutboxRetry:
		return w.handleJobRetry(ctx, operation)
	case providerops.KindPaymentRefund:
		return w.handlePaymentRefund(ctx, operation, job)
	default:
		return errors.New("unsupported administrator mutation operation")
	}
}

func (w *MutationWorker) handleJobRetry(ctx context.Context, operation providerops.Operation) error {
	operation, item, _, err := w.start(ctx, operation, "outbox_job")
	if err != nil {
		return err
	}
	err = w.repository.CompleteOutboxRetryOperation(ctx, operation.Receipt.ID, item.Key, item.TargetID, w.now().UTC())
	if errors.Is(err, database.ErrNotFound) {
		return w.complete(ctx, operation, item, providerops.StatusFailed, "OUTBOX_JOB_NOT_FOUND")
	}
	if errors.Is(err, database.ErrConflict) {
		return w.complete(ctx, operation, item, providerops.StatusFailed, "OUTBOX_JOB_NOT_RETRYABLE")
	}
	return err
}

var _ providerops.Handler = (*MutationWorker)(nil)
