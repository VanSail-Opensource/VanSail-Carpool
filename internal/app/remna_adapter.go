package app

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

type remnaClient interface {
	FindUserByUsername(context.Context, string) (*remnawave.User, bool, error)
	FindUserByTelegramID(context.Context, int64) (*remnawave.User, error)
	GetUserByID(context.Context, int64) (*remnawave.User, error)
	ListUsersStream(context.Context, string, int) ([]remnawave.User, *string, bool, error)
	CreateUser(context.Context, remnawave.CreateUserRequest) (*remnawave.User, error)
	GetUserStats(context.Context, int64, time.Time, time.Time, int) (*remnawave.UserStats, error)
	GetStatsDigest(context.Context, time.Time, time.Time) (remnawave.StatsDigest, error)
	GetNodesUsage(context.Context, time.Time, time.Time) (remnawave.NodesUsage, error)
	RequestUserConnections(context.Context, int64) (string, error)
	UserConnections(context.Context, string) (remnawave.ConnectionScan, error)
	DropConnectionByIP(context.Context, string, string) error
	BlockIP(context.Context, string, string, int) error
	UnblockIP(context.Context, string, string) error
	RevokeSubscription(context.Context, int64, bool) (*remnawave.User, error)
	UpdateUser(context.Context, remnawave.UpdateUserRequest) (*remnawave.User, error)
	DisableUser(context.Context, int64) error
	EnableUser(context.Context, int64) error
	ResetTraffic(context.Context, int64) (*remnawave.User, error)
	ListInternalSquads(context.Context) ([]remnawave.InternalSquad, error)
	ListNodes(context.Context) ([]remnawave.Node, error)
	RequestNodeGeocheck(context.Context, string) (string, error)
	NodeGeocheckResult(context.Context, string) (remnawave.NodeGeocheck, error)
	ListHosts(context.Context) ([]remnawave.Host, error)
	UpdateHostRemark(context.Context, string, string) error
	InternalSquadAccessibleNodes(context.Context, string) ([]remnawave.AccessibleNode, error)
}

type remnaClientFactory func(context.Context) (remnaClient, error)

type squadPreferenceSource interface {
	EnabledSquadsForRemote(context.Context, string, []string) ([]string, error)
}

type remnaAdapter struct {
	settings         *admin.SettingsService
	queue            *upstreamqueue.Queue
	clientFactory    remnaClientFactory
	multipliers      *nodeMultiplierCache
	squadPreferences squadPreferenceSource
}

func newRemnaAdapter(settings *admin.SettingsService, queue *upstreamqueue.Queue) remnaAdapter {
	return remnaAdapter{settings: settings, queue: queue, multipliers: newNodeMultiplierCache()}
}

func (a remnaAdapter) client(ctx context.Context) (remnaClient, error) {
	if a.clientFactory != nil {
		return a.clientFactory(ctx)
	}
	if a.settings == nil {
		return nil, errors.New("Remnawave settings are unavailable")
	}
	baseURL, err := a.settings.Plaintext(ctx, "remnawave.base_url")
	if err != nil {
		return nil, err
	}
	token, err := a.settings.Plaintext(ctx, "remnawave.api_token")
	if err != nil {
		return nil, err
	}
	return remnawave.NewClient(baseURL, token)
}

func remnaCall[T any](ctx context.Context, adapter remnaAdapter, call func(context.Context, remnaClient) (T, error)) (T, error) {
	return upstreamqueue.Do(ctx, adapter.queue, func(callCtx context.Context) (T, error) {
		client, err := adapter.client(callCtx)
		if err != nil {
			var zero T
			return zero, err
		}
		return call(callCtx, client)
	})
}

func remnaExecute(ctx context.Context, adapter remnaAdapter, call func(context.Context, remnaClient) error) error {
	return upstreamqueue.Execute(ctx, adapter.queue, func(callCtx context.Context) error {
		client, err := adapter.client(callCtx)
		if err != nil {
			return err
		}
		return call(callCtx, client)
	})
}

func remnaUserID(remoteID string) (int64, error) {
	userID, err := strconv.ParseInt(remoteID, 10, 64)
	if err != nil || userID <= 0 {
		return 0, errors.New("invalid Remnawave user id")
	}
	return userID, nil
}

func mapRemoteUser(user remnawave.User) accounts.RemoteUser {
	return accounts.RemoteUser{
		ID: strconv.FormatInt(user.ID, 10), Username: user.Username,
		TelegramID: user.TelegramID,
	}
}

func (a remnaAdapter) IsDuplicateError(err error) bool {
	return remnawave.IsErrorCode(err, "A019")
}

var _ remnaClient = (*remnawave.Client)(nil)
