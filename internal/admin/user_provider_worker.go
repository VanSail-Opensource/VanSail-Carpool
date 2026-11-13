package admin

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type adminUserProviderRepository interface {
	BeginProviderOperationAttempt(context.Context, string, time.Time) (providerops.Operation, error)
	CompleteProviderOperation(context.Context, string, providerops.Completion, time.Time) (providerops.Operation, error)
	UserByID(context.Context, string) (model.User, error)
	CompleteAdminTemporaryUnban(context.Context, string, string, time.Time) error
}

type adminUserProviderRemote interface {
	SetAdminUserDisabled(context.Context, model.User, string, bool) error
}

// UserProviderWorker executes isolated provider-backed account actions from the durable queue.
type UserProviderWorker struct {
	repository adminUserProviderRepository
	remote     adminUserProviderRemote
	now        func() time.Time
}

func NewUserProviderWorker(repository adminUserProviderRepository, remote adminUserProviderRemote) *UserProviderWorker {
	return &UserProviderWorker{repository: repository, remote: remote, now: time.Now}
}

func (w *UserProviderWorker) HandleProviderOperation(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	if operation.Receipt.Status == string(providerops.StatusQueued) {
		var err error
		operation, err = w.repository.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, w.now().UTC())
		if err != nil {
			return err
		}
	}
	if operation.Receipt.Status != string(providerops.StatusProcessing) {
		return nil
	}
	completion, err := w.run(ctx, operation, job)
	if err != nil {
		return err
	}
	_, err = w.repository.CompleteProviderOperation(ctx, operation.Receipt.ID, completion, w.now().UTC())
	return err
}

func (w *UserProviderWorker) run(ctx context.Context, operation providerops.Operation, job model.OutboxJob) (providerops.Completion, error) {
	var err error
	switch operation.Receipt.Kind {
	case providerops.KindAdminTemporaryBan:
		user, completion, loadErr := w.userForAction(ctx, operation.OwnerUserID)
		if loadErr != nil {
			return providerops.Completion{}, loadErr
		}
		if completion != nil {
			return *completion, nil
		}
		err = w.remote.SetAdminUserDisabled(ctx, user, operation.Receipt.ID, true)
	case providerops.KindAdminTemporaryUnban:
		user, completion, loadErr := w.userForAction(ctx, operation.OwnerUserID)
		if loadErr != nil {
			return providerops.Completion{}, loadErr
		}
		if completion != nil {
			return *completion, nil
		}
		err = w.remote.SetAdminUserDisabled(ctx, user, operation.Receipt.ID, false)
		if err == nil {
			err = w.repository.CompleteAdminTemporaryUnban(ctx, user.ID, operation.Receipt.ID, w.now().UTC())
		}
	default:
		return providerops.Completion{Status: providerops.StatusFailed, ErrorCode: "UNSUPPORTED_ADMIN_OPERATION"}, nil
	}
	if err != nil {
		return providerops.Completion{Status: providerops.StatusPendingReview, ErrorCode: "UPSTREAM_OUTCOME_AMBIGUOUS"}, nil
	}
	return providerops.Completion{Status: providerops.StatusSucceeded}, nil
}

func (w *UserProviderWorker) userForAction(ctx context.Context, userID string) (model.User, *providerops.Completion, error) {
	user, err := w.repository.UserByID(ctx, userID)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return model.User{}, nil, err
	}
	if err != nil || user.ID != userID {
		return model.User{}, &providerops.Completion{Status: providerops.StatusFailed, ErrorCode: "LOCAL_STATE_UNAVAILABLE"}, nil
	}
	return user, nil, nil
}

var _ providerops.Handler = (*UserProviderWorker)(nil)
