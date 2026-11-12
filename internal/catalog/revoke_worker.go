package catalog

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
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
	// A completed item is the immutable outcome of an earlier attempt. Recover
	// its parent receipt before reading mutable credentials or calling the provider.
	if providerops.Terminal(item.Status) {
		if item.Status == providerops.StatusSucceeded {
			s.InvalidateDashboard(item.TargetID)
		}
		completion := providerops.Completion{Status: item.Status, ErrorCode: item.ErrorCode,
			ProviderReference: item.ProviderReference, ResultJSON: item.ResultJSON}
		_, err := repository.CompleteProviderOperation(ctx, operation.Receipt.ID, completion, s.now().UTC())
		return err
	}
	if operation.OwnerUserID == "" || operation.ActorUserID != operation.OwnerUserID || item.TargetID != operation.OwnerUserID {
		return errors.New("subscription revoke owner does not match the command")
	}
	target, err := revokeOperationTarget(job)
	if err != nil {
		return err
	}
	user, err := repository.UserByID(ctx, operation.OwnerUserID)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return err
	}
	if err != nil {
		return s.finishRevoke(ctx, repository, operation, item, providerops.StatusFailed, "XNODE_IDENTITY_REQUIRED")
	}
	if user.ID != operation.OwnerUserID || user.AccountGeneration != target.Generation || user.AccountStatus != "active" {
		return s.finishRevoke(ctx, repository, operation, item, providerops.StatusFailed, "ACCOUNT_GENERATION_SUPERSEDED")
	}
	provider, ok := s.provider.(memberSubscriptionRevoker)
	if !ok {
		return errors.New("member subscription revocation is unavailable")
	}
	current, err := revokeDashboard(ctx, repository, provider, user)
	if errors.Is(err, database.ErrAccountUnavailable) {
		return s.finishRevoke(ctx, repository, operation, item, providerops.StatusFailed, "ACCOUNT_GENERATION_SUPERSEDED")
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(current.SubscriptionURL) == "" {
		return errors.New("current subscription credential is unavailable")
	}
	if item.Status == providerops.StatusProcessing {
		if subscriptionHash(current.SubscriptionURL) != target.PreviousHash {
			s.InvalidateDashboard(user.ID)
			return s.finishRevoke(ctx, repository, operation, item, providerops.StatusSucceeded, "")
		}
		return s.finishRevoke(ctx, repository, operation, item, providerops.StatusPendingReview, "REVOKE_OUTCOME_AMBIGUOUS")
	}
	operation, item, err = s.beginRevoke(ctx, repository, operation, item)
	if err != nil {
		return err
	}
	if subscriptionHash(current.SubscriptionURL) != target.PreviousHash {
		s.InvalidateDashboard(user.ID)
		return s.finishRevoke(ctx, repository, operation, item, providerops.StatusSucceeded, "")
	}
	_, callErr := provider.RevokeSubscriptionForMember(ctx, user)
	if callErr == nil {
		s.InvalidateDashboard(user.ID)
		return s.finishRevoke(ctx, repository, operation, item, providerops.StatusSucceeded, "")
	}
	if errors.Is(callErr, database.ErrAccountUnavailable) {
		return s.finishRevoke(ctx, repository, operation, item, providerops.StatusFailed, "ACCOUNT_GENERATION_SUPERSEDED")
	}
	after, reconcileErr := revokeDashboard(ctx, repository, provider, user)
	if errors.Is(reconcileErr, database.ErrAccountUnavailable) {
		return s.finishRevoke(ctx, repository, operation, item, providerops.StatusFailed, "ACCOUNT_GENERATION_SUPERSEDED")
	}
	if reconcileErr == nil && strings.TrimSpace(after.SubscriptionURL) != "" && subscriptionHash(after.SubscriptionURL) != target.PreviousHash {
		s.InvalidateDashboard(user.ID)
		return s.finishRevoke(ctx, repository, operation, item, providerops.StatusSucceeded, "")
	}
	if definitiveCatalogFailure(s.provider, callErr) {
		return s.finishRevoke(ctx, repository, operation, item, providerops.StatusFailed, "REVOKE_REJECTED")
	}
	return s.finishRevoke(ctx, repository, operation, item, providerops.StatusPendingReview, "REVOKE_OUTCOME_AMBIGUOUS")
}

// A credential read can race a purge and recreation even when the provider
// resolves the correct owner. Recheck the generation after the read so the new
// account's token can never serve as proof for the retired command.
func revokeDashboard(ctx context.Context, repository revokeOperationRepository, provider memberDashboardReader, admitted model.User) (RemoteDashboard, error) {
	current, err := provider.DashboardForMember(ctx, admitted)
	if err != nil {
		return RemoteDashboard{}, err
	}
	user, err := repository.UserByID(ctx, admitted.ID)
	if errors.Is(err, database.ErrNotFound) || err == nil && (user.ID != admitted.ID || user.AccountGeneration != admitted.AccountGeneration || user.AccountStatus != "active") {
		return RemoteDashboard{}, database.ErrAccountUnavailable
	}
	if err != nil {
		return RemoteDashboard{}, err
	}
	return current, nil
}

func revokeOperationTarget(job model.OutboxJob) (revokeTarget, error) {
	value, err := jobpayload.TargetID(job, "sealedTarget")
	if err != nil {
		return revokeTarget{}, err
	}
	var target revokeTarget
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	if len(value) > 1024 || decoder.Decode(&target) != nil || decoder.Decode(new(any)) != io.EOF || target.Generation < 0 {
		return revokeTarget{}, errors.New("subscription revoke target is invalid")
	}
	hash, err := hex.DecodeString(target.PreviousHash)
	if err != nil || len(hash) != 32 {
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
