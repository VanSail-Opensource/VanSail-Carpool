package app

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

var errBridgePurchaseResetUncertain = errors.New("bridge purchase reset requires reconciliation")

type purchaseResetReference struct {
	PurchaseID  string
	OperationID string
}

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
	a := r.provider
	unlock := a.lockBridgeRead()
	defer unlock()
	if operation != "" {
		applied, err := r.ResetOperationCompleted(ctx, id, r.purchase.ID, operation)
		if err != nil {
			return err
		}
		if applied {
			return nil
		}
	}
	if err := a.native.CheckPurchaseOperation(ctx, id, r.purchase.ID); err != nil {
		return err
	}
	resetNative := func() error {
		if operation != "" {
			return a.native.ResetPurchaseTrafficForOperation(ctx, id, r.purchase.ID, operation)
		}
		return a.native.ResetPurchaseTraffic(ctx, id, r.purchase.ID)
	}
	if r.identity.LegacyUserID == "" {
		return resetNative()
	}
	bridge, err := a.legacy(ctx)
	if err != nil {
		return err
	}
	if !bridge {
		return resetNative()
	}
	if err := a.resetBridge(ctx, r.identity, purchaseResetReference{PurchaseID: r.purchase.ID, OperationID: operation}); err != nil {
		// Source work may have committed before a period mismatch or lost reply.
		// Do not refund this charge merely because its native successor is current.
		return errors.Join(errBridgePurchaseResetUncertain, err)
	}
	return nil
}
