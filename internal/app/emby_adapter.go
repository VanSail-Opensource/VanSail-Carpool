package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	domain "github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	provider "github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

type embyClient interface {
	FindUserByName(context.Context, string) (domain.RemoteUser, bool, error)
	CreateUser(context.Context, string) (domain.RemoteUser, error)
	GetUser(context.Context, string) (domain.RemoteUser, error)
	SetPassword(context.Context, string, []byte, []byte) error
	UpdatePolicy(context.Context, string, domain.Policy) error
	ListSelectableFolders(context.Context) ([]domain.Folder, error)
	ListParentalRatings(context.Context) ([]domain.ParentalRating, error)
}

type embyClientFactory func(context.Context) (embyClient, error)

// embyAdapter resolves encrypted settings for every provider call so an
// administrator can rotate the endpoint or token without restarting.
type embyAdapter struct {
	settings      *admin.SettingsService
	queue         *upstreamqueue.Queue
	clientFactory embyClientFactory
}

func newEmbyAdapter(settings *admin.SettingsService, queue *upstreamqueue.Queue) embyAdapter {
	return embyAdapter{settings: settings, queue: queue}
}

func (a embyAdapter) client(ctx context.Context) (embyClient, error) {
	if a.clientFactory != nil {
		return a.clientFactory(ctx)
	}
	if a.settings == nil {
		return nil, errors.New("Emby settings are unavailable")
	}
	baseURL, err := a.settings.Plaintext(ctx, "emby.base_url")
	if err != nil {
		return nil, err
	}
	token, err := a.settings.Plaintext(ctx, "emby.api_token")
	if err != nil {
		return nil, err
	}
	return provider.NewClient(baseURL, token)
}

func embyCall[T any](ctx context.Context, adapter embyAdapter, call func(context.Context, embyClient) (T, error)) (T, error) {
	return upstreamqueue.Do(ctx, adapter.queue, func(callCtx context.Context) (T, error) {
		client, err := adapter.client(callCtx)
		if err != nil {
			var zero T
			return zero, err
		}
		return call(callCtx, client)
	})
}

func embyExecute(ctx context.Context, adapter embyAdapter, call func(context.Context, embyClient) error) error {
	return upstreamqueue.Execute(ctx, adapter.queue, func(callCtx context.Context) error {
		client, err := adapter.client(callCtx)
		if err != nil {
			return err
		}
		return call(callCtx, client)
	})
}

func (a embyAdapter) FindUserByName(ctx context.Context, name string) (domain.RemoteUser, bool, error) {
	type lookup struct {
		user   domain.RemoteUser
		exists bool
	}
	found, err := embyCall(ctx, a, func(callCtx context.Context, client embyClient) (lookup, error) {
		user, exists, callErr := client.FindUserByName(callCtx, name)
		return lookup{user: user, exists: exists}, callErr
	})
	return found.user, found.exists, err
}

func (a embyAdapter) CreateUser(ctx context.Context, name string) (domain.RemoteUser, error) {
	return embyCall(ctx, a, func(callCtx context.Context, client embyClient) (domain.RemoteUser, error) {
		return client.CreateUser(callCtx, name)
	})
}

func (a embyAdapter) GetUser(ctx context.Context, id string) (domain.RemoteUser, error) {
	return embyCall(ctx, a, func(callCtx context.Context, client embyClient) (domain.RemoteUser, error) {
		return client.GetUser(callCtx, id)
	})
}

func (a embyAdapter) SetPassword(ctx context.Context, id string, currentPassword, nextPassword []byte) error {
	current := append([]byte(nil), currentPassword...)
	next := append([]byte(nil), nextPassword...)
	defer clear(current)
	defer clear(next)
	return embyExecute(ctx, a, func(callCtx context.Context, client embyClient) error {
		return client.SetPassword(callCtx, id, current, next)
	})
}

func (a embyAdapter) UpdatePolicy(ctx context.Context, id string, policy domain.Policy) error {
	input := policy.Clone()
	return embyExecute(ctx, a, func(callCtx context.Context, client embyClient) error {
		return client.UpdatePolicy(callCtx, id, input)
	})
}

func (a embyAdapter) ListSelectableFolders(ctx context.Context) ([]domain.Folder, error) {
	return embyCall(ctx, a, func(callCtx context.Context, client embyClient) ([]domain.Folder, error) {
		return client.ListSelectableFolders(callCtx)
	})
}

func (a embyAdapter) ListParentalRatings(ctx context.Context) ([]domain.ParentalRating, error) {
	return embyCall(ctx, a, func(callCtx context.Context, client embyClient) ([]domain.ParentalRating, error) {
		return client.ListParentalRatings(callCtx)
	})
}

func (a embyAdapter) IsNotFound(err error) bool {
	var apiError *provider.APIError
	return errors.As(err, &apiError) && apiError.HTTPStatus == http.StatusNotFound
}

func (a embyAdapter) IsTerminal(err error) bool {
	var apiError *provider.APIError
	return errors.As(err, &apiError) && apiError.HTTPStatus >= 400 && apiError.HTTPStatus < 500 &&
		apiError.HTTPStatus != http.StatusRequestTimeout && apiError.HTTPStatus != http.StatusTooManyRequests
}

func embySetupPrice(settings *admin.SettingsService) domain.PriceSource {
	return domain.PriceFunc(func(ctx context.Context) (int64, error) {
		raw, err := settings.Plaintext(ctx, "emby.setup_price_txb")
		if err != nil {
			return 0, err
		}
		minor, err := billing.ParseTXBMajor(raw)
		if err != nil || minor < 0 {
			return 0, fmt.Errorf("invalid Emby setup price")
		}
		return minor, nil
	})
}

var _ domain.Remote = embyAdapter{}
var _ embyClient = (*provider.Client)(nil)
