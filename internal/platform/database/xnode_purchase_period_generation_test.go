package database

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestNativePurchaseMutationKeepsCapturedOwnerGeneration(t *testing.T) {
	for _, mode := range []string{"restore", "first", "replacement"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			status := "open"
			if mode == "first" {
				status = "absent"
			}
			f, ctx := newPurchasePeriodRestoreFixture(t, status), context.Background()
			target := f.purchase
			if mode == "replacement" {
				target = replacementPeriodPurchase(t, f)
			}
			if _, err := f.store.DB().ExecContext(ctx, `UPDATE users SET account_generation=1 WHERE id=$1`, f.identity.UserID); err != nil {
				t.Fatal(err)
			}
			var err error
			if mode == "restore" {
				err = f.store.RestoreXNodePurchasePeriod(ctx, f.identity, f.period, target, 0, f.now)
			} else {
				_, err = f.store.ActivateXNodePurchasePeriod(ctx, f.identity, f.period, target, "new-generation-period", 0, f.now)
			}
			if !errors.Is(err, ErrAccountUnavailable) {
				t.Fatal("stale generation crossed native mutation", err)
			}
			identity, err := f.store.XNodeIdentityByUser(ctx, f.identity.UserID)
			if err != nil || identity.CurrentPeriodID != f.identity.CurrentPeriodID {
				t.Fatal(identity, err)
			}
			if f.period.ID != "" {
				period, err := f.store.XNodePeriod(ctx, f.period.ID)
				if err != nil || !reflect.DeepEqual(period, f.period) {
					t.Fatal("stale generation changed evidence", period, err)
				}
			}
			if mode == "restore" {
				err = f.store.RestoreXNodePurchasePeriod(ctx, f.identity, f.period, target, 1, f.now)
			} else {
				_, err = f.store.ActivateXNodePurchasePeriod(ctx, f.identity, f.period, target, "new-generation-period", 1, f.now)
			}
			if err != nil {
				t.Fatal("current generation refused", err)
			}
		})
	}
}
