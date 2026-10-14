package app

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
)

type remnaLookup struct {
	user   *remnawave.User
	exists bool
}

func (a remnaAdapter) FindUserByUsername(ctx context.Context, username string) (accounts.RemoteUser, bool, error) {
	found, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (remnaLookup, error) {
		user, exists, err := client.FindUserByUsername(callCtx, username)
		return remnaLookup{user: user, exists: exists}, err
	})
	if err != nil || !found.exists {
		return accounts.RemoteUser{}, found.exists, err
	}
	return mapRemoteUser(*found.user), true, nil
}

func (a remnaAdapter) FindUserByTelegramID(ctx context.Context, telegramID int64) (accounts.RemoteUser, bool, error) {
	user, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (*remnawave.User, error) {
		return client.FindUserByTelegramID(callCtx, telegramID)
	})
	if err != nil {
		return accounts.RemoteUser{}, false, err
	}
	if user == nil {
		return accounts.RemoteUser{}, false, nil
	}
	return mapRemoteUser(*user), true, nil
}

func (a remnaAdapter) FindUserByID(ctx context.Context, remoteID string) (accounts.RemoteUser, bool, error) {
	userID, err := remnaUserID(remoteID)
	if err != nil {
		return accounts.RemoteUser{}, false, err
	}
	user, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (*remnawave.User, error) {
		return client.GetUserByID(callCtx, userID)
	})
	if remnawave.IsNotFound(err) {
		return accounts.RemoteUser{}, false, nil
	}
	if err != nil {
		return accounts.RemoteUser{}, false, err
	}
	return mapRemoteUser(*user), true, nil
}

func (a remnaAdapter) CreateUser(ctx context.Context, input accounts.RemoteCreateUser) (accounts.RemoteUser, error) {
	request := remnawave.CreateUserRequest{
		Username: input.Username, Status: remnawave.UserStatus(input.Status), TrafficLimitBytes: input.TrafficLimitBytes,
		TrafficLimitStrategy: remnawave.TrafficLimitStrategy(input.TrafficLimitStrategy), ExpireAt: input.ExpireAt,
		TelegramID: input.TelegramID, ActiveInternalSquads: append([]string(nil), input.ActiveInternalSquads...),
	}
	user, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (*remnawave.User, error) {
		return client.CreateUser(callCtx, request)
	})
	if err != nil {
		return accounts.RemoteUser{}, err
	}
	return mapRemoteUser(*user), nil
}

var _ accounts.RemnawaveClient = remnaAdapter{}
