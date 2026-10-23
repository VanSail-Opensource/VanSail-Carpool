package xnode_test

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestAccessRepairFindsOpenResetAfterSettledPurchasePeriod(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	identity, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	previous, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		t.Fatal(err)
	}
	previous.Status = "closed"
	if err = f.store.SaveXNodePeriod(ctx, previous); err != nil {
		t.Fatal(err)
	}
	next := previous
	next.ID, next.Status = "reset-open", "open"
	next.StartedAt = previous.StartedAt.Add(time.Second)
	if err = f.store.SaveXNodePeriod(ctx, next); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=12345 WHERE id=?`, next.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_identities SET current_period_id=? WHERE user_id=?`, previous.ID, f.user.ID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err = f.service.RepairAccess(ctx, model.OutboxJob{Payload: `{"userId":"` + f.user.ID + `"}`}); err != nil {
			t.Fatal(err)
		}
	}
	after, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
	if err != nil || after.CurrentPeriodID != next.ID {
		t.Fatalf("restored pointer = %q, %v", after.CurrentPeriodID, err)
	}
	periods, err := f.store.XNodePeriods(ctx, identity.ProxyUserID)
	if err != nil || len(periods) != 2 || periods[0].Status != "closed" || periods[1].UsedBytes != 12345 {
		t.Fatalf("purchase history changed: %+v, %v", periods, err)
	}
	if _, err = f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "Paid host", Address: "proxy.example", Port: 443, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	metadata, err := f.service.OwnedSubscription(ctx, f.user.ID)
	if err != nil || !metadata.Active || metadata.Incomplete || metadata.UsedBytes != "12345" {
		t.Fatalf("repaired subscription = %+v, %v", metadata, err)
	}
}
