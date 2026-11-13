package database

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func replacementPeriodPurchase(t *testing.T, f purchasePeriodRestoreFixture) model.Purchase {
	t.Helper()
	ctx := context.Background()
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE purchases SET status='expired' WHERE id=$1`, f.purchase.ID); err != nil {
		t.Fatal(err)
	}
	next, err := f.store.CreatePurchase(ctx, PurchaseInput{UserID: f.identity.UserID, ComboID: f.purchase.ComboID, IdempotencyKey: "new-paid-term"}, f.now)
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func TestPurchasePeriodActivationCreatesExactPaidTermAndRetainsPredecessor(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "first", true: "replacement"}[existing], func(t *testing.T) {
			t.Parallel()
			status := "absent"
			if existing {
				status = "open"
			}
			f, ctx := newPurchasePeriodRestoreFixture(t, status), context.Background()
			target := f.purchase
			if existing {
				target = replacementPeriodPurchase(t, f)
			}
			closed, err := f.store.ActivateXNodePurchasePeriod(ctx, f.identity, f.period, target, "new-period", 0, f.now)
			if err != nil {
				t.Fatal(err)
			}
			identity, err := f.store.XNodeIdentityByUser(ctx, f.identity.UserID)
			if err != nil || identity.CurrentPeriodID != "new-period" {
				t.Fatal(identity, err)
			}
			period, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
			if err != nil || period.PurchaseID != target.ID || period.Status != "open" || period.UsedBytes != 0 || period.LimitBytes != target.TrafficLimitBytes {
				t.Fatal(period, err)
			}
			if existing {
				previous, err := f.store.XNodePeriod(ctx, f.period.ID)
				if err != nil || previous.Status != "closing" || previous.UsedBytes != 77 || previous.ClosureGeneration != f.period.ClosureGeneration+1 || !reflect.DeepEqual(previous, closed) {
					t.Fatal("predecessor evidence replaced", previous, closed, err)
				}
			} else if closed.ID != "" {
				t.Fatal("first activation invented predecessor", closed)
			}
			if _, err = f.store.ActivateXNodePurchasePeriod(ctx, f.identity, f.period, target, "replayed-period", 0, f.now); !errors.Is(err, xnode.ErrConflict) {
				t.Fatal("stale pointer admitted second quota", err)
			}
		})
	}
}

func TestPurchasePeriodActivationRollsBackPredecessorWhenInsertionFails(t *testing.T) {
	t.Parallel()
	f, ctx := newPurchasePeriodRestoreFixture(t, "open"), context.Background()
	target := replacementPeriodPurchase(t, f)
	if _, err := f.store.ActivateXNodePurchasePeriod(ctx, f.identity, f.period, target, f.period.ID, 0, f.now); err == nil {
		t.Fatal("duplicate period id succeeded")
	}
	previous, err := f.store.XNodePeriod(ctx, f.period.ID)
	if err != nil || !reflect.DeepEqual(previous, f.period) {
		t.Fatal("failed admission closed predecessor", previous, err)
	}
	identity, err := f.store.XNodeIdentityByUser(ctx, f.identity.UserID)
	if err != nil || identity.CurrentPeriodID != f.period.ID {
		t.Fatal("failed admission changed pointer", identity, err)
	}
}
