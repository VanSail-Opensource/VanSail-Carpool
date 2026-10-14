package emby

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (s *OperationService) handlePreferences(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	target, err := s.decodeTarget(job, operation)
	if err != nil {
		return err
	}
	operation, item, fresh, err := s.start(ctx, operation, "emby_user")
	if err != nil {
		return err
	}
	if !fresh {
		return s.reconcilePreferences(ctx, operation, item, target.Preferences)
	}
	if _, err := s.core.UpdatePreferences(ctx, operation.OwnerUserID, target.Preferences); err == nil {
		return s.complete(ctx, operation, item, providerops.StatusSucceeded, "")
	} else if applied, reconcileErr := s.preferencesApplied(ctx, operation.OwnerUserID, target.Preferences); reconcileErr == nil && applied {
		return s.complete(ctx, operation, item, providerops.StatusSucceeded, "")
	} else if s.core.remote.IsTerminal(err) {
		return s.complete(ctx, operation, item, providerops.StatusFailed, "EMBY_PREFERENCES_REJECTED")
	}
	return s.complete(ctx, operation, item, providerops.StatusPendingReview, "EMBY_PREFERENCES_AMBIGUOUS")
}

func (s *OperationService) reconcilePreferences(ctx context.Context, operation providerops.Operation, item providerops.Item,
	preferences Preferences) error {
	applied, err := s.preferencesApplied(ctx, operation.OwnerUserID, preferences)
	if err == nil && applied {
		return s.complete(ctx, operation, item, providerops.StatusSucceeded, "")
	}
	return s.complete(ctx, operation, item, providerops.StatusPendingReview, "EMBY_PREFERENCES_AMBIGUOUS")
}

func (s *OperationService) preferencesApplied(ctx context.Context, userID string, preferences Preferences) (bool, error) {
	account, err := s.core.repository.EmbyAccountForUser(ctx, userID)
	if err != nil {
		return false, err
	}
	remote, err := s.core.remote.GetUser(ctx, account.RemoteUserID)
	if err != nil || !PolicyMatchesPreferences(remote.Policy, preferences) {
		return false, err
	}
	_, err = s.core.repository.UpdateEmbyPreferences(ctx, account.ID, preferences, s.now().UTC())
	return err == nil, err
}

func (s *OperationService) handlePassword(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	target, err := s.decodeTarget(job, operation)
	if err != nil {
		return err
	}
	operation, item, fresh, err := s.start(ctx, operation, "emby_user")
	if err != nil {
		return err
	}
	if !fresh {
		return s.complete(ctx, operation, item, providerops.StatusPendingReview, "EMBY_PASSWORD_AMBIGUOUS")
	}
	if err := s.core.ChangePassword(ctx, operation.OwnerUserID, "", target.Password); err == nil {
		return s.complete(ctx, operation, item, providerops.StatusSucceeded, "")
	} else if s.core.remote.IsTerminal(err) {
		return s.complete(ctx, operation, item, providerops.StatusFailed, "EMBY_PASSWORD_REJECTED")
	}
	return s.complete(ctx, operation, item, providerops.StatusPendingReview, "EMBY_PASSWORD_AMBIGUOUS")
}
