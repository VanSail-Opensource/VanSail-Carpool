package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestMaintenanceRetainsNativeRoutingAndProbeReceipts(t *testing.T) {
	t.Parallel()
	store, owner, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()
	route, _, err := store.CreateProviderOperation(ctx, providerops.CreateInput{
		ActorUserID: owner.UserID, OwnerUserID: owner.UserID, Kind: "xnode_routing",
		IdempotencyKey: "retention-route", RequestFingerprint: "retention-route-fingerprint",
		Items: []providerops.ItemInput{{Key: "routing", TargetType: "user", TargetID: owner.UserID}},
	}, at)
	if err != nil {
		t.Fatal(err)
	}
	completeTestProviderOperation(t, store, route.Receipt.ID, at)
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO xnode_routing_plans
		(proxy_user_id,cipher,revision,phase,operation_id,updated_at) VALUES(?,'encrypted',1,'active',?,?)`,
		owner.ProxyUserID, route.Receipt.ID, stamp(at)); err != nil {
		t.Fatal(err)
	}
	probe := meteredProbeFixture(t, store, owner, period, "retention-probe")
	completeTestProviderOperation(t, store, probe.OperationID, at)
	now := at.Add(8 * 24 * time.Hour)
	for range 2 {
		if _, err := store.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now); err != nil {
			t.Fatal("repeat maintenance must retain referenced receipts", err)
		}
		var retained int
		if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM provider_operations WHERE id IN (?,?)`,
			route.Receipt.ID, probe.OperationID).Scan(&retained); err != nil || retained != 2 {
			t.Fatal("native receipt attribution lost", retained, err)
		}
	}
}
