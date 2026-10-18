package rollover

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// PurchaseRemote resolves native identity from the owner and selects accounting
// by purchase ID. A missing legacy account is not evidence of zero native usage.
type PurchaseRemote interface {
	QuiescePurchaseForRollover(context.Context, model.User, model.Purchase) error
	PurchaseUsageSnapshotForRollover(context.Context, model.User, model.Purchase, time.Time) (UsageSnapshot, error)
}

// PurchaseSnapshot is shared by live projection and final settlement.
func PurchaseSnapshot(ctx context.Context, remote any, user model.User, purchase model.Purchase, end time.Time) (UsageSnapshot, error) {
	if native, ok := remote.(PurchaseRemote); ok {
		return native.PurchaseUsageSnapshotForRollover(ctx, user, purchase, end)
	}
	legacy, ok := remote.(interface {
		UsageSnapshotForRollover(context.Context, string, time.Time, time.Time) (UsageSnapshot, error)
	})
	if !ok || user.RemnaUserID == nil {
		return UsageSnapshot{}, ErrRemoteUserMissing
	}
	return legacy.UsageSnapshotForRollover(ctx, *user.RemnaUserID, purchase.ValidFrom, end)
}

func quiescePurchase(ctx context.Context, remote Remote, user model.User, purchase model.Purchase) error {
	if native, ok := remote.(PurchaseRemote); ok {
		return native.QuiescePurchaseForRollover(ctx, user, purchase)
	}
	if user.RemnaUserID == nil {
		return ErrRemoteUserMissing
	}
	if period, ok := remote.(PeriodRemote); ok {
		return period.QuiesceRolloverPeriod(ctx, *user.RemnaUserID, purchase.ValidFrom, purchase.ValidUntil)
	}
	return remote.QuiesceForRollover(ctx, *user.RemnaUserID)
}
