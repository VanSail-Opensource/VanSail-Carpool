package app

import (
	"context"
	"errors"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type purchaseScopedProvider struct {
	provider proxyProvider
	identity xnode.Identity
	purchase model.Purchase
}

func (a proxyProvider) PurchaseRemote(ctx context.Context, user model.User, purchase model.Purchase) (purchaseops.Remote, string, error) {
	if user.ID == "" || purchase.ID == "" || purchase.UserID != user.ID {
		return nil, "", xnode.ErrInvalid
	}
	identity, err := a.native.IdentityForUser(ctx, user.ID)
	if err == nil {
		if identity.UserID != user.ID {
			return nil, "", xnode.ErrConflict
		}
		return purchaseScopedProvider{provider: a, identity: identity, purchase: purchase}, strconv.FormatInt(identity.ProxyUserID, 10), nil
	}
	if !errors.Is(err, xnode.ErrNotFound) {
		return nil, "", err
	}
	return nil, "", purchaseops.ErrRemoteIdentityMissing
}

func (r purchaseScopedProvider) validID(id string) bool {
	return id == strconv.FormatInt(r.identity.ProxyUserID, 10)
}

func (r purchaseScopedProvider) MemberOperationState(ctx context.Context, id string) (purchaseops.RemoteState, error) {
	if !r.validID(id) {
		return purchaseops.RemoteState{}, xnode.ErrInvalid
	}
	if err := r.provider.native.CheckPurchaseOperation(ctx, id, r.purchase.ID); err != nil {
		return purchaseops.RemoteState{}, err
	}
	return r.provider.native.PurchaseOperationState(ctx, id, r.purchase.ID)
}

func (r purchaseScopedProvider) MemberRefundState(ctx context.Context, id, purchase string) (purchaseops.RemoteState, error) {
	if !r.validID(id) || purchase != r.purchase.ID {
		return purchaseops.RemoteState{}, xnode.ErrInvalid
	}
	return r.provider.MemberRefundState(ctx, id, purchase)
}

func (r purchaseScopedProvider) QuiesceMemberOperation(ctx context.Context, id string) error {
	if !r.validID(id) {
		return xnode.ErrInvalid
	}
	return r.provider.native.QuiescePurchaseOperation(ctx, id, r.purchase.ID)
}

func (r purchaseScopedProvider) RestoreMemberOperation(ctx context.Context, id string, purchase model.Purchase) error {
	if !r.validID(id) || purchase.ID != r.purchase.ID || purchase.UserID != r.identity.UserID {
		return xnode.ErrInvalid
	}
	if err := r.provider.native.CheckPurchaseOperation(ctx, id, purchase.ID); err != nil {
		return err
	}
	return r.provider.RestoreMemberOperation(ctx, id, purchase)
}

func (r purchaseScopedProvider) DefinitiveMutationFailure(err error) bool {
	return r.provider.DefinitiveMutationFailure(err)
}
