package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestWakeRoutingRetainsReceiptsAndRepairsOnlyUnfinishedJobs(t *testing.T) {
	for _, tc := range []struct {
		name, kind, status, operationStatus string
		remove, wake                        bool
	}{
		{"missing routing", xnode.RoutingOperationKind, "", "processing", true, true},
		{"missing DIY", xnode.DIYOperationKind, "", "queued", true, true},
		{"failed", xnode.RoutingOperationKind, "failed", "processing", false, true},
		{"backoff", xnode.DIYOperationKind, "pending", "processing", false, true},
		{"leased", xnode.DIYOperationKind, "processing", "processing", false, false},
		{"terminal missing", xnode.RoutingOperationKind, "", "succeeded", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, identity, _ := xnodeTrafficFixture(t)
			ctx, now := context.Background(), time.Now().UTC()
			input := providerops.CreateInput{ActorUserID: identity.UserID, OwnerUserID: identity.UserID, Kind: tc.kind, IdempotencyKey: "recovery", RequestFingerprint: "routing-recovery-fingerprint", Items: []providerops.ItemInput{{Key: "routing", TargetType: "xnode_user", TargetID: fmt.Sprint(identity.ProxyUserID)}}}
			receipt, err := store.QueueXNodeRouting(ctx, xnode.RoutingPlan{ProxyUserID: identity.ProxyUserID, Cipher: "encrypted"}, input, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.db.ExecContext(ctx, `UPDATE provider_operations SET status=?,result_json='{"diySave":{"settingsRevision":7}}' WHERE id=?`, tc.operationStatus, receipt.ID); err != nil {
				t.Fatal(err)
			}
			if tc.remove {
				_, err = store.db.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE json_extract(payload,'$.operationId')=?`, receipt.ID)
			} else {
				_, err = store.db.ExecContext(ctx, `UPDATE outbox_jobs SET status=?,attempts=8,available_at=? WHERE json_extract(payload,'$.operationId')=?`, tc.status, stamp(now.Add(time.Hour)), receipt.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err = store.WakeXNodeRouting(ctx, now); err != nil {
					t.Fatal(err)
				}
			}
			var count, attempts int
			var status string
			if err = store.db.QueryRowContext(ctx, `SELECT count(*),coalesce(min(status),''),coalesce(min(attempts),0) FROM outbox_jobs WHERE json_extract(payload,'$.operationId')=?`, receipt.ID).Scan(&count, &status, &attempts); err != nil {
				t.Fatal(err)
			}
			if tc.wake && (count != 1 || status != "pending" || attempts != 0) {
				t.Fatalf("recovery did not reuse one runnable job: %d %s %d", count, status, attempts)
			}
			if !tc.wake && (tc.remove && count != 0 || !tc.remove && (status != tc.status || attempts != 8)) {
				t.Fatal("recovery changed leased or terminal work")
			}
			var result, operationStatus string
			if err = store.db.QueryRowContext(ctx, `SELECT status,result_json FROM provider_operations WHERE id=?`, receipt.ID).Scan(&operationStatus, &result); err != nil || operationStatus != tc.operationStatus || result != `{"diySave":{"settingsRevision":7}}` {
				t.Fatal("recovery changed receipt or financial evidence", err)
			}
		})
	}
}
