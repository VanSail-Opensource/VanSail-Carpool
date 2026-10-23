package xnode_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestCadenceRecoversInterruptedSuccessorWithoutRewritingUsage(t *testing.T) {
	for _, scenario := range []string{"closing", "closed", "review", "early-closure", "refund-pending"} {
		t.Run(scenario, func(t *testing.T) {
			f := controlFixture(t, true)
			ctx := context.Background()
			identity, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
			if err != nil {
				t.Fatal(err)
			}
			started := time.Now().UTC().Add(-49 * time.Hour)
			ended := started.Add(25 * time.Hour)
			status := scenario
			if scenario == "early-closure" {
				status, ended = "closed", started.Add(time.Hour)
			}
			if scenario == "refund-pending" {
				status = "closing"
				if _, _, err = f.store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: f.user.ID, OwnerUserID: f.user.ID, Kind: "purchase_refund", IdempotencyKey: "pending-refund", RequestFingerprint: strings.Repeat("a", 64), SealedTarget: "encrypted"}, time.Now().UTC()); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET started_at=?,last_reset_at=?,ended_at=?,strategy='DAY',status=?,used_bytes=12345 WHERE id=?`, started.Format(time.RFC3339Nano), started.Format(time.RFC3339Nano), ended.Format(time.RFC3339Nano), status, identity.CurrentPeriodID); err != nil {
				t.Fatal(err)
			}
			if err = f.service.ResetCadences(ctx); err != nil {
				t.Fatal(err)
			}
			after, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
			if err != nil {
				t.Fatal(err)
			}
			wantRecovery := scenario == "closing" || scenario == "closed"
			if (after.CurrentPeriodID != identity.CurrentPeriodID) != wantRecovery {
				t.Fatalf("recovery = %v, want %v", after.CurrentPeriodID != identity.CurrentPeriodID, wantRecovery)
			}
			old, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
			if err != nil || old.UsedBytes != 12345 || !old.StartedAt.Equal(started) || old.EndedAt == nil || !old.EndedAt.Equal(ended) {
				t.Fatalf("previous evidence changed: %+v, %v", old, err)
			}
			if !wantRecovery {
				if old.Status != status {
					t.Fatalf("fenced period changed status: %s", old.Status)
				}
				return
			}
			next, err := f.store.XNodePeriod(ctx, after.CurrentPeriodID)
			if err != nil || next.Status != "open" || next.UsedBytes != 0 || next.PurchaseID != old.PurchaseID || !next.StartedAt.Equal(started.Add(48*time.Hour)) {
				t.Fatalf("successor = %+v, %v", next, err)
			}
			if err = f.service.ResetCadences(ctx); err != nil {
				t.Fatal(err)
			}
			replay, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
			if err != nil || replay.CurrentPeriodID != after.CurrentPeriodID {
				t.Fatalf("recovery replay replaced successor: %+v, %v", replay, err)
			}
			if _, err = f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "Paid host", Address: "proxy.example", Port: 443, Enabled: true}); err != nil {
				t.Fatal(err)
			}
			metadata, err := f.service.OwnedSubscription(ctx, f.user.ID)
			if err != nil || !metadata.Active || metadata.Incomplete {
				t.Fatalf("recovered subscription = %+v, %v", metadata, err)
			}
		})
	}
}
