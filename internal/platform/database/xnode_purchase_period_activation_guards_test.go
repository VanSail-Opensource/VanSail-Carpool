package database

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestPurchasePeriodActivationRejectsStaleTargetBeforeClosingPredecessor(t *testing.T) {
	for _, scenario := range []string{"same-clock edit", "cancelled", "due", "disabled", "purging", "new pointer", "new closure", "pending refund"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			f, ctx := newPurchasePeriodRestoreFixture(t, "open"), context.Background()
			target := replacementPeriodPurchase(t, f)
			at := f.now
			var err error
			switch scenario {
			case "same-clock edit":
				_, err = f.store.DB().ExecContext(ctx, `UPDATE purchases SET entitlement_traffic_limit_bytes=2000 WHERE id=$1`, target.ID)
			case "cancelled":
				_, err = f.store.DB().ExecContext(ctx, `UPDATE purchases SET status='cancelled' WHERE id=$1`, target.ID)
			case "due":
				at = target.ValidUntil
			case "disabled":
				_, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_identities SET access_disabled=1 WHERE proxy_user_id=$1`, f.identity.ProxyUserID)
			case "purging":
				_, err = f.store.DB().ExecContext(ctx, `UPDATE users SET account_status='purging' WHERE id=$1`, f.identity.UserID)
			case "new pointer":
				other := f.period
				other.ID = "newer-current"
				err = f.store.SaveXNodePeriod(ctx, other)
			case "new closure":
				_, err = f.store.BeginXNodePeriodClosure(ctx, f.period, at.Add(time.Second))
			case "pending refund":
				_, _, err = f.store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: f.identity.UserID, OwnerUserID: f.identity.UserID, Kind: "purchase_refund", IdempotencyKey: "activation-refund", RequestFingerprint: "activation-refund",
					Items: []providerops.ItemInput{{Key: "purchase", TargetType: "purchase", TargetID: target.ID}}}, at)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := f.store.XNodePeriod(ctx, f.period.ID)
			if err != nil {
				t.Fatal(err)
			}
			pointer, err := f.store.XNodeIdentityByUser(ctx, f.identity.UserID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.store.ActivateXNodePurchasePeriod(ctx, f.identity, f.period, target, "rejected-period", 0, at); err == nil {
				t.Fatal("stale activation succeeded")
			}
			after, err := f.store.XNodePeriod(ctx, f.period.ID)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("rejected target changed predecessor", before, after, err)
			}
			actual, err := f.store.XNodeIdentityByUser(ctx, f.identity.UserID)
			if err != nil || actual.CurrentPeriodID != pointer.CurrentPeriodID {
				t.Fatal("rejected target changed pointer", actual, err)
			}
			var count int
			if err = f.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_periods WHERE id='rejected-period'`).Scan(&count); err != nil || count != 0 {
				t.Fatal("rejected target published fresh quota", count, err)
			}
		})
	}
}
