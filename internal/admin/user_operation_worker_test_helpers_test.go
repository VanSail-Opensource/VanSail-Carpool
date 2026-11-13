package admin

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type userOperationRepositoryStub struct {
	operation  providerops.Operation
	items      []providerops.Item
	users      map[string]model.User
	desired    map[string]*model.Purchase
	userErr    error
	desiredErr error
}

func (r *userOperationRepositoryStub) BeginProviderOperationAttempt(_ context.Context, _ string, now time.Time) (providerops.Operation, error) {
	r.operation.Receipt.Status = string(providerops.StatusProcessing)
	r.operation.Attempts++
	r.operation.AttemptStartedAt = &now
	return r.operation, nil
}

func (r *userOperationRepositoryStub) CompleteProviderOperation(_ context.Context, _ string,
	completion providerops.Completion, now time.Time) (providerops.Operation, error) {
	r.operation.Receipt.Status = string(completion.Status)
	r.operation.Receipt.UpdatedAt = now
	r.operation.Receipt.CompletedAt = &now
	if completion.ErrorCode != "" {
		r.operation.Receipt.ErrorCode = &completion.ErrorCode
	}
	r.operation.ResultJSON = completion.ResultJSON
	return r.operation, nil
}

func (r *userOperationRepositoryStub) ListProviderOperationItems(context.Context, string) ([]providerops.Item, error) {
	return append([]providerops.Item(nil), r.items...), nil
}

func (r *userOperationRepositoryStub) BeginProviderOperationItemAttempt(_ context.Context, _, key string,
	now time.Time) (providerops.Item, error) {
	for index := range r.items {
		if r.items[index].Key == key && r.items[index].Status == providerops.StatusQueued {
			r.items[index].Status = providerops.StatusProcessing
			r.items[index].AttemptStartedAt = &now
			return r.items[index], nil
		}
	}
	return providerops.Item{}, errors.New("item not queued")
}

func (r *userOperationRepositoryStub) CompleteProviderOperationItem(_ context.Context, _, key string,
	completion providerops.Completion, now time.Time) (providerops.Item, error) {
	for index := range r.items {
		if r.items[index].Key == key {
			r.items[index].Status = completion.Status
			r.items[index].ErrorCode = completion.ErrorCode
			r.items[index].ResultJSON = completion.ResultJSON
			r.items[index].CompletedAt = &now
			return r.items[index], nil
		}
	}
	return providerops.Item{}, errors.New("item missing")
}

func (r *userOperationRepositoryStub) UserByID(_ context.Context, id string) (model.User, error) {
	if r.userErr != nil {
		return model.User{}, r.userErr
	}
	user, ok := r.users[id]
	if !ok {
		return model.User{}, errors.New("user missing")
	}
	return user, nil
}

func (r *userOperationRepositoryStub) DesiredEntitlement(_ context.Context, id string, _ time.Time) (*model.Purchase, error) {
	if r.desiredErr != nil {
		return nil, r.desiredErr
	}
	desired, ok := r.desired[id]
	if !ok {
		return nil, errors.New("desired state missing")
	}
	return desired, nil
}

type entitlementRemoteStub struct {
	applyCalls []string
	failures   map[string]error
}

func (r *entitlementRemoteStub) SyncAdminUserEntitlement(_ context.Context, user model.User, _ *model.Purchase) error {
	r.applyCalls = append(r.applyCalls, user.ID)
	return r.failures[user.ID]
}
