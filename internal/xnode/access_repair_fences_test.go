package xnode_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestRepairAndSchedulerRespectRefundOwner(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	now := time.Now().UTC()
	identity, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	period, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		t.Fatal(err)
	}
	period.Status = "closing"
	period.EndedAt = &now
	if err = f.store.SaveXNodePeriod(ctx, period); err != nil {
		t.Fatal(err)
	}
	_, _, err = f.store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: f.user.ID, OwnerUserID: f.user.ID, Kind: "purchase_refund", IdempotencyKey: "refund-review", RequestFingerprint: "refund-fingerprint", SealedTarget: "sealed", Items: []providerops.ItemInput{{Key: "purchase", TargetType: "purchase", TargetID: period.PurchaseID}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	job := model.OutboxJob{Payload: `{"userId":"` + f.user.ID + `"}`}
	if err = f.service.RepairAccess(ctx, job); !errors.Is(err, outbox.ErrBusy) {
		t.Fatalf("refund ownership bypassed: %v", err)
	}
	if err = f.store.EnqueueXNodeAccessRepairs(ctx, now); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = f.store.DB().QueryRow(`SELECT COUNT(*) FROM outbox_jobs WHERE kind=?`, xnode.AccessRepairJob).Scan(&count); err != nil || count != 0 {
		t.Fatalf("blocked repair enqueued: %d %v", count, err)
	}
	after, err := f.store.XNodePeriod(ctx, period.ID)
	if err != nil || after.Status != "closing" {
		t.Fatalf("refund period resumed: %+v %v", after, err)
	}
}

func TestTransactionalRepairRejectsStalePointer(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	identity, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	period, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		t.Fatal(err)
	}
	purchase, err := f.service.EffectivePurchase(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	stale := identity
	stale.CurrentPeriodID = "obsolete-pointer"
	if err = f.store.RepairXNodeAccessPeriod(ctx, stale, *purchase, period, time.Now(), false); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("stale pointer accepted: %v", err)
	}
	after, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
	if err != nil || after.CurrentPeriodID != identity.CurrentPeriodID {
		t.Fatal("new pointer overwritten", err)
	}
}
