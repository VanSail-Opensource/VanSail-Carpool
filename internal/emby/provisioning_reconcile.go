package emby

import (
	"context"
	"fmt"
	"strings"
)

func (s *Service) resolveOrCreate(ctx context.Context, record *ProvisioningRecord) (RemoteUser, error) {
	if record.RemoteUserID != "" {
		user, err := s.remote.GetUser(ctx, record.RemoteUserID)
		if err == nil {
			return user, nil
		}
		if !s.remote.IsNotFound(err) {
			return RemoteUser{}, err
		}
		// The stored id may have been lost after an Emby restore. Reconcile by
		// the persisted exact candidate before considering the setup terminal.
		if record.CandidateUsername != "" {
			user, exists, findErr := s.remote.FindUserByName(ctx, record.CandidateUsername)
			if findErr != nil {
				return RemoteUser{}, findErr
			}
			if exists {
				return user, nil
			}
		}
		return RemoteUser{}, &remoteAbsenceProof{record: *record, cause: ErrRemoteAccountMissing}
	}

	if record.CandidateUsername == "" {
		candidate, err := s.chooseCandidate(ctx, record.BaseUsername, record.UserID)
		if err != nil {
			return RemoteUser{}, err
		}
		if err := s.repository.SetEmbyCandidate(ctx, record.ID, candidate, s.now().UTC()); err != nil {
			return RemoteUser{}, err
		}
		record.CandidateUsername = candidate
	}
	if record.CreateAttempted {
		user, exists, err := s.remote.FindUserByName(ctx, record.CandidateUsername)
		if err != nil {
			return RemoteUser{}, err
		}
		if exists {
			return user, nil
		}
	} else {
		_, exists, err := s.remote.FindUserByName(ctx, record.CandidateUsername)
		if err != nil {
			return RemoteUser{}, err
		}
		if exists {
			return RemoteUser{}, fmt.Errorf("%w: candidate appeared before create", ErrAccountExists)
		}
	}
	if err := s.repository.MarkEmbyCreateAttempted(ctx, record.ID, s.now().UTC()); err != nil {
		return RemoteUser{}, err
	}
	record.CreateAttempted = true
	created, createErr := s.remote.CreateUser(ctx, record.CandidateUsername)
	if createErr == nil {
		return created, nil
	}
	if s.remote.IsTerminal(createErr) {
		return RemoteUser{}, s.confirmRejectedCreate(ctx, *record, fmt.Errorf("create Emby user: %w", createErr))
	}
	// Emby can commit creation and lose the response. The candidate was
	// persisted and preflighted before this call, so exact-name reconciliation
	// is the only identity adoption allowed.
	reconciled, exists, reconcileErr := s.remote.FindUserByName(ctx, record.CandidateUsername)
	if reconcileErr == nil && exists {
		return reconciled, nil
	}
	if reconcileErr != nil && !s.remote.IsTerminal(createErr) {
		return RemoteUser{}, fmt.Errorf("create Emby user: %w; reconcile: %v", createErr, reconcileErr)
	}
	return RemoteUser{}, fmt.Errorf("create Emby user: %w", createErr)
}

func (s *Service) chooseCandidate(ctx context.Context, baseUsername, userID string) (string, error) {
	baseUsername = strings.TrimSpace(baseUsername)
	if baseUsername == "" {
		return "", fmt.Errorf("%w: empty candidate username", ErrInvalidSetup)
	}
	_, exists, err := s.remote.FindUserByName(ctx, baseUsername)
	if err != nil {
		return "", fmt.Errorf("preflight Emby username: %w", err)
	}
	if !exists {
		return baseUsername, nil
	}
	suffixed := SuffixedUsername(baseUsername, userID)
	_, exists, err = s.remote.FindUserByName(ctx, suffixed)
	if err != nil {
		return "", fmt.Errorf("preflight suffixed Emby username: %w", err)
	}
	if exists {
		return "", fmt.Errorf("%w: generated Emby username already exists", ErrAccountExists)
	}
	return suffixed, nil
}

func (s *Service) validatePreferences(ctx context.Context, preferences Preferences) error {
	options, err := s.Options(ctx)
	if err != nil {
		return err
	}
	availableFolders := make(map[string]struct{}, len(options.Folders))
	for _, folder := range options.Folders {
		availableFolders[folder.ID] = struct{}{}
	}
	for _, selected := range preferences.DisabledLibraryIDs {
		if _, ok := availableFolders[selected]; !ok {
			return fmt.Errorf("%w: unknown Emby library", ErrInvalidSetup)
		}
	}
	if preferences.MaxParentalRating != nil {
		found := false
		for _, rating := range options.Ratings {
			if rating.Value == *preferences.MaxParentalRating {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: unknown Emby parental rating", ErrInvalidSetup)
		}
	}
	return nil
}
