package accounts

import (
	"context"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// BeginAgreement advances onboarding without reserving a service username.
func (s *Service) BeginAgreement(ctx context.Context, user model.User) (model.User, error) {
	repo, ok := s.repository.(interface {
		BeginAgreement(context.Context, string) error
	})
	if !ok {
		return model.User{}, errors.New("onboarding repository unavailable")
	}
	if err := repo.BeginAgreement(ctx, user.ID); err != nil {
		return model.User{}, err
	}
	return s.repository.UserByID(ctx, user.ID)
}

// AcceptAgreementRevision validates the published agreements and completes the
// local account. Proxy provisioning begins only after a combo purchase.
func (s *Service) AcceptAgreementRevision(ctx context.Context, user model.User, revision int, agreementIDs []string) (model.User, error) {
	if user.OnboardingState != "agreement" || revision <= 0 {
		return model.User{}, fmt.Errorf("%w: %w", ErrAgreementStateConflict, database.ErrConflict)
	}
	currentRevision, requiredIDs, err := s.repository.CurrentAgreementContract(ctx)
	if err != nil {
		return model.User{}, err
	}
	if revision != currentRevision || !sameStringSet(requiredIDs, agreementIDs) {
		return model.User{}, fmt.Errorf("%w: %w", ErrAgreementRevisionConflict, database.ErrConflict)
	}
	return s.repository.CompleteOnboardingRevision(ctx, user.ID, revision, agreementIDs, s.now().UTC())
}

func sameStringSet(expected, provided []string) bool {
	if len(expected) != len(provided) {
		return false
	}
	seen := make(map[string]struct{}, len(expected))
	for _, value := range expected {
		seen[value] = struct{}{}
	}
	for _, value := range provided {
		if _, exists := seen[value]; !exists {
			return false
		}
		delete(seen, value)
	}
	return len(seen) == 0
}
