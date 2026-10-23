package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// HandleProviderOperation rotates credentials once and reconciles any interrupted attempt.
func (s *Service) HandleProviderOperation(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	if operation.Receipt.Kind != providerops.KindSubscriptionRevoke {
		return errors.New("unsupported catalog provider operation")
	}
	repository, ok := s.repository.(revokeOperationRepository)
	if !ok {
		return errors.New("durable subscription revocation is unavailable")
	}
	item, err := revokeItem(ctx, repository, operation.Receipt.ID)
	if err != nil {
		return err
	}
	target, err := revokeOperationTarget(job)
	if err != nil {
		return err
	}
	user, err := repository.UserByID(ctx, item.TargetID)
	if err != nil || user.ProxyUserID == nil {
		return s.finishRevoke(ctx, repository, operation, item, providerops.StatusFailed, "XNODE_IDENTITY_REQUIRED")
	}
	current, err := s.provider.Dashboard(ctx, *user.ProxyUserID)
	if err != nil {
		return err
	}
	if item.Status == providerops.StatusProcessing {
		if subscriptionHash(current.SubscriptionURL) != target.PreviousHash {
			s.invalidateDashboard(user.ID)
			return s.finishRevoke(ctx, repository, operation, item, providerops.StatusSucceeded, "")
		}
		return s.finishRevoke(ctx, repository, operation, item, providerops.StatusPendingReview, "REVOKE_OUTCOME_AMBIGUOUS")
	}
	operation, item, err = s.beginRevoke(ctx, repository, operation, item)
	if err != nil {
		return err
	}
	if subscriptionHash(current.SubscriptionURL) != target.PreviousHash {
		s.invalidateDashboard(user.ID)
		return s.finishRevoke(ctx, repository, operation, item, providerops.StatusSucceeded, "")
	}
	_, callErr := s.provider.RevokeSubscription(ctx, *user.ProxyUserID)
	if callErr == nil {
		s.invalidateDashboard(user.ID)
		return s.finishRevoke(ctx, repository, operation, item, providerops.StatusSucceeded, "")
	}
	after, reconcileErr := s.provider.Dashboard(ctx, *user.ProxyUserID)
	if reconcileErr == nil && subscriptionHash(after.SubscriptionURL) != target.PreviousHash {
		s.invalidateDashboard(user.ID)
		return s.finishRevoke(ctx, repository, operation, item, providerops.StatusSucceeded, "")
	}
	if definitiveCatalogFailure(s.provider, callErr) {
		return s.finishRevoke(ctx, repository, operation, item, providerops.StatusFailed, "REVOKE_REJECTED")
	}
	return s.finishRevoke(ctx, repository, operation, item, providerops.StatusPendingReview, "REVOKE_OUTCOME_AMBIGUOUS")
}

func revokeOperationTarget(job model.OutboxJob) (revokeTarget, error) {
	value, err := jobpayload.TargetID(job, "sealedTarget")
	if err != nil {
		return revokeTarget{}, err
	}
	var target revokeTarget
	if json.Unmarshal([]byte(value), &target) != nil || len(target.PreviousHash) != 64 {
		return revokeTarget{}, errors.New("subscription revoke target is invalid")
	}
	return target, nil
}

func definitiveCatalogFailure(remote ProxyClient, err error) bool {
	type classifier interface{ DefinitiveMutationFailure(error) bool }
	if value, ok := remote.(classifier); ok {
		return value.DefinitiveMutationFailure(err)
	}
	var status interface{ HTTPStatusCode() int }
	return errors.As(err, &status) && status.HTTPStatusCode() >= http.StatusBadRequest && status.HTTPStatusCode() < http.StatusInternalServerError
}

var _ providerops.Handler = (*Service)(nil)
