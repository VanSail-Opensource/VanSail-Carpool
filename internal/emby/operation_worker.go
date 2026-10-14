package emby

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// HandleProviderOperation applies one Emby command without blindly replaying an ambiguous write.
func (s *OperationService) HandleProviderOperation(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	switch operation.Receipt.Kind {
	case providerops.KindEmbySetup:
		return s.handleSetup(ctx, operation, job)
	case providerops.KindEmbyPreferences:
		return s.handlePreferences(ctx, operation, job)
	case providerops.KindEmbyPassword:
		return s.handlePassword(ctx, operation, job)
	case providerops.KindEmbyProvisionRetry:
		return s.handleProvisionRetry(ctx, operation)
	default:
		return errors.New("unsupported Emby operation kind")
	}
}

func (s *OperationService) handleSetup(ctx context.Context, operation providerops.Operation, _ model.OutboxJob) error {
	operation, item, _, err := s.start(ctx, operation, "emby_user")
	if err != nil {
		return err
	}
	account, err := s.core.repository.EmbyAccountForUser(ctx, operation.OwnerUserID)
	if err != nil {
		return err
	}
	return s.advanceProvisioning(ctx, operation, item, account.ID)
}

func (s *OperationService) handleProvisionRetry(ctx context.Context, operation providerops.Operation) error {
	operation, item, _, err := s.start(ctx, operation, "emby_account")
	if err != nil {
		return err
	}
	record, err := s.core.repository.EmbyProvisioningByID(ctx, item.TargetID)
	if err != nil {
		return s.complete(ctx, operation, item, providerops.StatusFailed, "EMBY_RETRY_REJECTED")
	}
	switch record.Status {
	case StatusActive:
		return s.complete(ctx, operation, item, providerops.StatusSucceeded, "")
	case StatusFailed:
		return s.complete(ctx, operation, item, providerops.StatusCompensated, "EMBY_SETUP_REFUNDED")
	case StatusQueued, StatusProvisioning:
		return s.advanceProvisioning(ctx, operation, item, record.ID)
	default:
		return s.complete(ctx, operation, item, providerops.StatusFailed, "EMBY_RETRY_REJECTED")
	}
}

func (s *OperationService) advanceProvisioning(ctx context.Context, operation providerops.Operation, item providerops.Item,
	accountID string) error {
	err := s.core.Provision(ctx, accountID)
	if errors.Is(err, ErrPendingReview) {
		return s.complete(ctx, operation, item, providerops.StatusPendingReview, "EMBY_OUTCOME_AMBIGUOUS")
	}
	if err != nil {
		return err
	}
	account, err := s.core.repository.EmbyProvisioningByID(ctx, accountID)
	if err != nil {
		return err
	}
	switch account.Status {
	case StatusActive:
		return s.complete(ctx, operation, item, providerops.StatusSucceeded, "")
	case StatusFailed:
		return s.complete(ctx, operation, item, providerops.StatusCompensated, "EMBY_SETUP_REFUNDED")
	case StatusPendingReview:
		return s.complete(ctx, operation, item, providerops.StatusPendingReview, "EMBY_OUTCOME_AMBIGUOUS")
	default:
		return errors.New("Emby provisioning did not reach a durable outcome")
	}
}

var _ providerops.Handler = (*OperationService)(nil)
