package app

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
	"time"
)

func (a proxyProvider) QuiescePurchaseForRollover(ctx context.Context, user model.User, purchase model.Purchase) error {
	return a.native.QuiescePurchaseForRollover(ctx, user, purchase)
}
func (a proxyProvider) PurchaseUsageSnapshotForRollover(ctx context.Context, user model.User, purchase model.Purchase, end time.Time) (rollover.UsageSnapshot, error) {
	return a.native.PurchaseUsageSnapshotForRollover(ctx, user, purchase, end)
}
