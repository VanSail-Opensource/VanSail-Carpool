package emby

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

const maxPasswordBytes = 1024

// Service coordinates priced setup, durable provisioning, and linked updates.
type Service struct {
	repository Repository
	remote     Remote
	prices     PriceSource
	secrets    SecretBox
	now        func() time.Time
}

// NewService constructs an Emby account service.
func NewService(repository Repository, remote Remote, prices PriceSource, secrets SecretBox) *Service {
	return &Service{repository: repository, remote: remote, prices: prices, secrets: secrets, now: time.Now}
}

// Account returns the caller's safe local Emby account state.
func (s *Service) Account(ctx context.Context, userID string) (Account, error) {
	return s.repository.EmbyAccountForUser(ctx, userID)
}

// ListAccounts returns safe account states for administrative monitoring.
func (s *Service) ListAccounts(ctx context.Context, limit int) ([]Account, error) {
	return s.repository.ListEmbyAccounts(ctx, limit)
}

// RetryProvisioning re-enqueues a retained transient setup by account id.
// Refunded terminal failures cannot be retried because their secret is erased.
func (s *Service) RetryProvisioning(ctx context.Context, accountID string) (Account, error) {
	return s.repository.RetryEmbyProvisioning(ctx, accountID, s.now().UTC())
}

// Options retrieves the current selectable folders and parental ratings.
func (s *Service) Options(ctx context.Context) (Options, error) {
	folders, err := s.remote.ListSelectableFolders(ctx)
	if err != nil {
		return Options{}, fmt.Errorf("list Emby folders: %w", err)
	}
	ratings, err := s.remote.ListParentalRatings(ctx)
	if err != nil {
		return Options{}, fmt.Errorf("list Emby parental ratings: %w", err)
	}
	return Options{Folders: folders, Ratings: ratings}, nil
}

// Setup validates current Emby choices, seals the temporary password, and
// atomically debits the setup price while queueing provisioning.
func (s *Service) Setup(ctx context.Context, userID, password string, preferences Preferences) (Account, bool, error) {
	if strings.TrimSpace(userID) == "" || len(password) == 0 || len(password) > maxPasswordBytes {
		return Account{}, false, ErrInvalidSetup
	}
	existing, err := s.repository.EmbyAccountForUser(ctx, userID)
	if err == nil && existing.Status != StatusFailed {
		return existing, false, nil
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Account{}, false, err
	}
	input, err := s.prepareSetup(ctx, userID, password, preferences)
	if err != nil {
		return Account{}, false, err
	}
	account, created, err := s.repository.QueueEmbySetup(ctx, input, s.now().UTC())
	if err != nil {
		return Account{}, false, err
	}
	return account, created, nil
}

func (s *Service) prepareSetup(ctx context.Context, userID, password string,
	preferences Preferences) (QueueSetupInput, error) {
	if strings.TrimSpace(userID) == "" || len(password) == 0 || len(password) > maxPasswordBytes {
		return QueueSetupInput{}, ErrInvalidSetup
	}
	preferences = normalizePreferences(preferences)
	if err := s.validatePreferences(ctx, preferences); err != nil {
		return QueueSetupInput{}, err
	}
	baseUsername, err := s.repository.EmbyBaseUsername(ctx, userID)
	if err != nil {
		return QueueSetupInput{}, fmt.Errorf("load Emby base username: %w", err)
	}
	baseUsername = strings.TrimSpace(baseUsername)
	if baseUsername == "" {
		return QueueSetupInput{}, fmt.Errorf("%w: local username is empty", ErrInvalidSetup)
	}
	price, err := s.prices.EmbySetupPriceTXBMinor(ctx)
	if err != nil {
		return QueueSetupInput{}, fmt.Errorf("load Emby setup price: %w", err)
	}
	if price < 0 {
		return QueueSetupInput{}, fmt.Errorf("%w: setup price is negative", ErrInvalidSetup)
	}
	accountID, err := ids.New()
	if err != nil {
		return QueueSetupInput{}, err
	}
	passwordBytes := []byte(password)
	defer zero(passwordBytes)
	secretContext := passwordContext(userID)
	ciphertext, err := s.secrets.Seal(secretContext, passwordBytes)
	if err != nil {
		return QueueSetupInput{}, fmt.Errorf("seal Emby provisioning password: %w", err)
	}
	return QueueSetupInput{
		ID: accountID, UserID: userID, BaseUsername: baseUsername,
		PasswordCiphertext: ciphertext, PasswordContext: secretContext,
		SetupPriceTXBMinor: price, Preferences: preferences,
	}, nil
}
