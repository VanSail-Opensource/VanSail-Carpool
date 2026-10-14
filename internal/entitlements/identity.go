package entitlements

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// IdentityClient exposes only the queued provider identity operations.
type IdentityClient interface {
	accounts.RemnawaveClient
	FindUserByID(context.Context, string) (accounts.RemoteUser, bool, error)
}

type provisioningRepository interface {
	HasProvisionablePurchase(context.Context, string, time.Time) (bool, error)
	LinkProvisionedRemnaUser(context.Context, string, string, string, string, time.Time) error
	ResolveProvisioningConflict(context.Context, string, string, time.Time) error
	QueueRemnawaveRepair(context.Context, string, string, time.Time) (model.User, error)
}

var errIdentityUnverified = errors.New("Remnawave identity cannot be verified")

func matchesIdentity(remote accounts.RemoteUser, user model.User) bool {
	id, err := strconv.ParseInt(remote.ID, 10, 64)
	return err == nil && id > 0 && user.Username != nil && remote.Username == *user.Username &&
		remote.TelegramID != nil && *remote.TelegramID == user.TelegramID
}

func identityMismatch(remote accounts.RemoteUser, user model.User) error {
	if user.Username != nil && remote.Username == *user.Username && remote.TelegramID != nil && *remote.TelegramID != user.TelegramID {
		return accounts.ErrRemnawaveIdentityConflict
	}
	return errIdentityUnverified
}

func (w *Worker) ensureIdentity(ctx context.Context, user model.User) (model.User, error) {
	if w.identity == nil {
		return user, nil
	}
	if user.OnboardingState != "complete" || user.Username == nil || *user.Username == "" {
		return user, errors.New("completed user has no reserved Remnawave username")
	}
	repository, ok := w.repository.(provisioningRepository)
	if !ok {
		return user, errors.New("Remnawave provisioning repository is unavailable")
	}
	previousID := ""
	if user.RemnaUserID != nil {
		previousID = *user.RemnaUserID
		remote, exists, err := w.identity.FindUserByID(ctx, previousID)
		if err != nil {
			return user, fmt.Errorf("verify linked Remnawave user: %w", err)
		}
		if exists {
			if !matchesIdentity(remote, user) {
				return user, identityMismatch(remote, user)
			}
			return user, nil
		}
	}
	remote, exists, err := w.identity.FindUserByUsername(ctx, *user.Username)
	if err != nil {
		return user, fmt.Errorf("find Remnawave username: %w", err)
	}
	if exists && !matchesIdentity(remote, user) {
		return user, identityMismatch(remote, user)
	}
	if !exists {
		byTelegram, found, lookupErr := w.identity.FindUserByTelegramID(ctx, user.TelegramID)
		if lookupErr != nil {
			return user, fmt.Errorf("find Remnawave Telegram identity: %w", lookupErr)
		}
		if found {
			if !matchesIdentity(byTelegram, user) {
				return user, identityMismatch(byTelegram, user)
			}
			remote, exists = byTelegram, true
		}
	}
	if !exists {
		remote, err = w.identity.CreateUser(ctx, accounts.RemoteCreateUser{
			Username: *user.Username, TelegramID: user.TelegramID, Status: "DISABLED",
			ExpireAt:          time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC),
			TrafficLimitBytes: 0, TrafficLimitStrategy: "NO_RESET", ActiveInternalSquads: []string{},
		})
		if err != nil && w.identity.IsDuplicateError(err) {
			remote, exists, err = w.identity.FindUserByUsername(ctx, *user.Username)
			if err == nil && !exists {
				return user, errors.New("Remnawave duplicate could not be reconciled")
			}
			if err == nil && !matchesIdentity(remote, user) {
				return user, identityMismatch(remote, user)
			}
		}
		if err != nil {
			return user, fmt.Errorf("create Remnawave user: %w", err)
		}
	}
	if !matchesIdentity(remote, user) {
		return user, identityMismatch(remote, user)
	}
	if err := repository.LinkProvisionedRemnaUser(ctx, user.ID, *user.Username, previousID, remote.ID, w.now().UTC()); err != nil {
		return user, err
	}
	user.RemnaUserID = &remote.ID
	return user, nil
}

func (w *Worker) resolveIdentityConflict(ctx context.Context, user model.User) error {
	if user.Username == nil {
		return errors.New("conflicted Remnawave user has no username")
	}
	repository, ok := w.repository.(provisioningRepository)
	if !ok {
		return errors.New("Remnawave conflict repository is unavailable")
	}
	return repository.ResolveProvisioningConflict(ctx, user.ID, *user.Username, w.now().UTC())
}

func (w *Worker) repairMissingIdentity(ctx context.Context, user model.User, failure error) error {
	if !remnawave.IsNotFound(failure) || user.RemnaUserID == nil || w.identity == nil {
		return failure
	}
	_, exists, err := w.identity.FindUserByID(ctx, *user.RemnaUserID)
	if err != nil || exists {
		return errors.Join(failure, err)
	}
	repository, ok := w.repository.(provisioningRepository)
	if !ok {
		return failure
	}
	_, err = repository.QueueRemnawaveRepair(ctx, user.ID, *user.RemnaUserID, w.now().UTC())
	return errors.Join(failure, err)
}
