package app

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"strconv"
)

// proxyProvider adapts the authoritative native control plane to commerce.
type proxyProvider struct {
	native *xnode.Service
	master *xnode.Master
	store  *database.Store
}

func remoteIdentity(identity xnode.Identity, user model.User) accounts.RemoteUser {
	telegramID := user.TelegramID
	return accounts.RemoteUser{ID: strconv.FormatInt(identity.ProxyUserID, 10), Username: user.AccountName(), TelegramID: &telegramID}
}
func (a proxyProvider) FindUserByUsername(ctx context.Context, name string) (accounts.RemoteUser, bool, error) {
	user, err := a.store.UserByUsername(ctx, name)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return accounts.RemoteUser{}, false, err
	}
	if err == nil {
		identity, e := a.native.IdentityForUser(ctx, user.ID)
		if e == nil {
			return remoteIdentity(identity, user), true, nil
		}
		if !errors.Is(e, xnode.ErrNotFound) {
			return accounts.RemoteUser{}, false, e
		}
	}
	identity, err := a.native.ServiceIdentityByName(ctx, name)
	if errors.Is(err, xnode.ErrNotFound) {
		return accounts.RemoteUser{}, false, nil
	}
	if err != nil {
		return accounts.RemoteUser{}, false, err
	}
	return accounts.RemoteUser{ID: strconv.FormatInt(identity.ProxyUserID, 10), Username: identity.ServiceName}, true, nil
}
func (a proxyProvider) FindUserByTelegramID(ctx context.Context, id int64) (accounts.RemoteUser, bool, error) {
	user, err := a.store.UserByTelegramID(ctx, id)
	if err != nil {
		return accounts.RemoteUser{}, false, err
	}
	identity, err := a.native.IdentityForUser(ctx, user.ID)
	if errors.Is(err, xnode.ErrNotFound) {
		return accounts.RemoteUser{}, false, nil
	}
	if err != nil {
		return accounts.RemoteUser{}, false, err
	}
	return remoteIdentity(identity, user), true, nil
}
func (a proxyProvider) FindUserByID(ctx context.Context, id string) (accounts.RemoteUser, bool, error) {
	identity, err := a.native.Identity(ctx, id)
	if errors.Is(err, xnode.ErrNotFound) {
		return accounts.RemoteUser{}, false, nil
	}
	if err != nil {
		return accounts.RemoteUser{}, false, err
	}
	if identity.UserID == "" && identity.ServiceName != "" {
		return accounts.RemoteUser{ID: strconv.FormatInt(identity.ProxyUserID, 10), Username: identity.ServiceName}, true, nil
	}
	user, err := a.native.IdentityUser(ctx, identity)
	if err != nil {
		return accounts.RemoteUser{}, false, err
	}
	return remoteIdentity(identity, user), true, nil
}
func (a proxyProvider) CreateUser(ctx context.Context, input accounts.RemoteCreateUser) (accounts.RemoteUser, error) {
	user, err := a.store.UserByTelegramID(ctx, input.TelegramID)
	if err != nil {
		return accounts.RemoteUser{}, err
	}
	if user.TelegramID != input.TelegramID {
		return accounts.RemoteUser{}, xnode.ErrConflict
	}
	identity, err := a.native.EnsureIdentity(ctx, user)
	return remoteIdentity(identity, user), err
}
func (a proxyProvider) IsDuplicateError(err error) bool { return errors.Is(err, xnode.ErrConflict) }
