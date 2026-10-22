package app

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"strconv"
	"sync"
)

// proxyProvider keeps the legacy integration available only during migration.
// Native identities remain authoritative even while some nodes use the bridge.
type proxyProvider struct {
	remnaAdapter
	native     *xnode.Service
	master     *xnode.Master
	store      *database.Store
	bridgeGate *sync.RWMutex
}

func (a proxyProvider) legacy(ctx context.Context) (bool, error) {
	state, err := a.store.XNodeMigration(ctx)
	if err != nil {
		return false, err
	}
	if state.Phase == "complete" {
		return false, nil
	}
	if state.SourceCipher != "" {
		return true, nil
	}
	source, err := a.settings.Optional(ctx, "remnawave.base_url")
	return source != "", err
}
func (a proxyProvider) nativeIdentity(ctx context.Context, id string) (xnode.Identity, bool, error) {
	identity, err := a.native.Identity(ctx, id)
	if err == nil {
		return identity, true, nil
	}
	if !errors.Is(err, xnode.ErrNotFound) {
		return identity, false, err
	}
	bridge, err := a.legacy(ctx)
	if err != nil {
		return identity, false, err
	}
	if !bridge {
		return identity, false, xnode.ErrNotFound
	}
	return identity, false, nil
}
func remoteIdentity(identity xnode.Identity, user model.User) accounts.RemoteUser {
	telegramID := user.TelegramID
	name := user.AccountName()
	return accounts.RemoteUser{ID: strconv.FormatInt(identity.ProxyUserID, 10), Username: name, TelegramID: &telegramID}
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
	service, serviceErr := a.native.ServiceIdentityByName(ctx, name)
	if serviceErr == nil {
		return accounts.RemoteUser{ID: strconv.FormatInt(service.ProxyUserID, 10), Username: service.ServiceName}, true, nil
	}
	if !errors.Is(serviceErr, xnode.ErrNotFound) {
		return accounts.RemoteUser{}, false, serviceErr
	}
	legacy, e := a.legacy(ctx)
	if e != nil {
		return accounts.RemoteUser{}, false, e
	}
	if legacy {
		return a.remnaAdapter.FindUserByUsername(ctx, name)
	}
	return accounts.RemoteUser{}, false, nil
}
func (a proxyProvider) FindUserByTelegramID(ctx context.Context, id int64) (accounts.RemoteUser, bool, error) {
	user, err := a.store.UserByTelegramID(ctx, id)
	if err != nil {
		return accounts.RemoteUser{}, false, err
	}
	identity, err := a.native.IdentityForUser(ctx, user.ID)
	if err == nil {
		return remoteIdentity(identity, user), true, nil
	}
	if !errors.Is(err, xnode.ErrNotFound) {
		return accounts.RemoteUser{}, false, err
	}
	bridge, err := a.legacy(ctx)
	if err != nil || !bridge {
		return accounts.RemoteUser{}, false, err
	}
	return a.remnaAdapter.FindUserByTelegramID(ctx, id)
}
func (a proxyProvider) FindUserByID(ctx context.Context, id string) (accounts.RemoteUser, bool, error) {
	identity, native, err := a.nativeIdentity(ctx, id)
	if err != nil {
		return accounts.RemoteUser{}, false, err
	}
	if !native {
		return a.remnaAdapter.FindUserByID(ctx, id)
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
	unlock := a.lockBridgeRead()
	defer unlock()
	legacy, err := a.legacy(ctx)
	if err != nil {
		return accounts.RemoteUser{}, err
	}
	if legacy {
		remote, err := a.remnaAdapter.CreateUser(ctx, input)
		if err != nil {
			return remote, err
		}
		user, err := a.store.UserByTelegramID(ctx, input.TelegramID)
		if err != nil {
			return remote, err
		}
		return a.importBridgeIdentity(ctx, user, remote)
	}
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
func (a proxyProvider) importBridgeIdentity(ctx context.Context, user model.User, remote accounts.RemoteUser) (accounts.RemoteUser, error) {
	if !matchesPanelIdentity(user, remote) {
		return accounts.RemoteUser{}, xnode.ErrConflict
	}
	id, err := strconv.ParseInt(remote.ID, 10, 64)
	if err != nil {
		return remote, err
	}
	legacy, err := remnaCall(ctx, a.remnaAdapter, func(ctx context.Context, client remnaClient) (*remnawave.User, error) {
		return client.GetUserByID(ctx, id)
	})
	if err != nil {
		return remote, err
	}
	if legacy == nil || legacy.ID != id || legacy.TelegramID == nil || *legacy.TelegramID != user.TelegramID {
		return remote, xnode.ErrConflict
	}
	identity, err := a.native.ImportIdentity(ctx, user, id, legacy.VLESSUUID, legacy.TrojanPassword, legacy.ShadowsocksPassword)
	return remoteIdentity(identity, user), err
}
func matchesPanelIdentity(user model.User, remote accounts.RemoteUser) bool {
	return remote.TelegramID != nil && *remote.TelegramID == user.TelegramID
}
func (a proxyProvider) IsDuplicateError(err error) bool {
	return errors.Is(err, xnode.ErrConflict) || a.remnaAdapter.IsDuplicateError(err)
}
