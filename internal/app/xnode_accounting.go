package app

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usageaccounting"
)

func (a proxyProvider) ClosePurchasePeriods(ctx context.Context, user model.User, purchase model.Purchase) error {
	return a.native.ClosePurchasePeriods(ctx, user, purchase)
}
func (a proxyProvider) PurchaseUsageSnapshot(ctx context.Context, user model.User, purchase model.Purchase, end time.Time) (usageaccounting.UsageSnapshot, error) {
	return a.native.PurchaseUsageSnapshot(ctx, user, purchase, end)
}
