package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type purchasePeriodRestoreFixture struct {
	store    *Store
	identity xnode.Identity
	period   xnode.Period
	purchase model.Purchase
	now      time.Time
}

func newPurchasePeriodRestoreFixture(t *testing.T, status string) purchasePeriodRestoreFixture {
	t.Helper()
	ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
	user := createTestUser(t, store, 89310)
	combo := saveTestCombo(t, store, "Retained native term", 100, 30)
	if _, err := store.AdjustBalance(ctx, user.ID, 1000, "retained-period-seed", "fixture", now); err != nil {
		t.Fatal(err)
	}
	purchase, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "retained-period"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.MarkPurchaseSyncResult(ctx, purchase.ID, true, now); err != nil {
		t.Fatal(err)
	}
	purchase, err = store.PurchaseByID(ctx, purchase.ID)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := store.SaveXNodeIdentity(ctx, xnode.Identity{UserID: user.ID, CredentialsCipher: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if status == "absent" {
		return purchasePeriodRestoreFixture{store: store, identity: identity, purchase: purchase, now: now}
	}
	period := xnode.Period{ID: "retained-period", ProxyUserID: identity.ProxyUserID, PurchaseID: purchase.ID, StartedAt: now, LastResetAt: &now, LimitBytes: purchase.TrafficLimitBytes, Strategy: purchase.ResetStrategy, Status: "open"}
	if err = store.SaveXNodePeriod(ctx, period); err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=77 WHERE id=$1`, period.ID); err != nil {
		t.Fatal(err)
	}
	if status != "open" {
		period, err = store.BeginXNodePeriodClosure(ctx, period, now)
		if err != nil {
			t.Fatal(err)
		}
		if status == "closed" {
			if err = store.FinalizeXNodePeriod(ctx, period); err != nil {
				t.Fatal(err)
			}
		}
	}
	period, err = store.XNodePeriod(ctx, period.ID)
	if err != nil {
		t.Fatal(err)
	}
	identity, err = store.XNodeIdentityByUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	return purchasePeriodRestoreFixture{store, identity, period, purchase, now}
}
