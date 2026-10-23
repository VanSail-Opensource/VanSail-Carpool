package purchaseops

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

var ErrRemoteIdentityMissing = errors.New("purchase provider identity is unavailable")

// PurchaseRemoteSource binds mutations and reconciliation to an owned purchase.
// Legacy providers without this extension retain their existing identity path.
type PurchaseRemoteSource interface {
	PurchaseRemote(context.Context, model.User, model.Purchase) (Remote, string, error)
}

// ResetOperationRemote proves a particular reset from its durable operation
// reference, independently of natural cadence resets and timestamp precision.
type ResetOperationRemote interface {
	ResetTrafficForOperation(context.Context, string, string) error
	ResetOperationCompleted(context.Context, string, string, string) (bool, error)
}

func purchaseRemote(ctx context.Context, remote Remote, user model.User, purchase model.Purchase) (Remote, string, error) {
	if source, ok := remote.(PurchaseRemoteSource); ok {
		return source.PurchaseRemote(ctx, user, purchase)
	}
	if user.ProxyUserID == nil || *user.ProxyUserID == "" {
		return nil, "", ErrRemoteIdentityMissing
	}
	return remote, *user.ProxyUserID, nil
}
