package app

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (r purchaseScopedProvider) ResetTraffic(ctx context.Context, id string) error {
	return r.resetTraffic(ctx, id, "")
}

func (r purchaseScopedProvider) ResetTrafficForOperation(ctx context.Context, id, operation string) error {
	return r.resetTraffic(ctx, id, operation)
}

func (r purchaseScopedProvider) ResetOperationCompleted(ctx context.Context, id, purchase, operation string) (bool, error) {
	if !r.validID(id) || purchase != r.purchase.ID {
		return false, xnode.ErrInvalid
	}
	return r.provider.native.ResetOperationCompleted(ctx, id, purchase, operation)
}

func (r purchaseScopedProvider) resetTraffic(ctx context.Context, id, operation string) error {
	if !r.validID(id) {
		return xnode.ErrInvalid
	}
	if operation != "" {
		applied, err := r.ResetOperationCompleted(ctx, id, r.purchase.ID, operation)
		if err != nil {
			return err
		}
		if applied {
			return nil
		}
	}
	if err := r.provider.native.CheckPurchaseOperation(ctx, id, r.purchase.ID); err != nil {
		return err
	}
	if operation != "" {
		return r.provider.native.ResetPurchaseTrafficForOperation(ctx, id, r.purchase.ID, operation)
	}
	return r.provider.native.ResetPurchaseTraffic(ctx, id, r.purchase.ID)
}
