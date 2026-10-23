package app

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

// MemberRefundState retains purchase-wide usage and native settlement barriers.
func (a proxyProvider) MemberRefundState(ctx context.Context, id, purchase string) (purchaseops.RemoteState, error) {
	return a.native.MemberRefundState(ctx, id, purchase)
}
