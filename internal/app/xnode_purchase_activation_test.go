package app

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/entitlements"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestNativeActivationReplayPreservesQuotaWithoutLegacyOwnerLink(t *testing.T) {
	for _, stale := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "foreign"}[stale], func(t *testing.T) {
			t.Parallel()
			f, ctx := newNativePurchaseFixture(t), context.Background()
			other := anotherNativeSyncOwner(t, f)
			identity, err := f.provider.native.IdentityForUser(ctx, f.user.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=77 WHERE id=$1`, identity.CurrentPeriodID); err != nil {
				t.Fatal(err)
			}
			if stale {
				if _, err = f.store.DB().ExecContext(ctx, `UPDATE users SET proxy_user_id=$1 WHERE id=$2`, strconv.FormatInt(other.ProxyUserID, 10), f.user.ID); err != nil {
					t.Fatal(err)
				}
			}
			payload, err := json.Marshal(map[string]string{"purchaseId": f.purchase.ID})
			if err != nil {
				t.Fatal(err)
			}
			worker := entitlements.NewWorker(f.store, f.provider, f.provider)
			job := model.OutboxJob{Kind: "xnode_apply_entitlement", Payload: string(payload), Attempts: 10}
			for range 2 {
				if err = worker.HandleOutbox(ctx, job); err != nil {
					t.Fatal(err)
				}
			}
			current, err := f.provider.native.IdentityForUser(ctx, f.user.ID)
			if err != nil || current.CurrentPeriodID != identity.CurrentPeriodID {
				t.Fatal("activation replaced retained period", current, err)
			}
			period, err := f.store.XNodePeriod(ctx, current.CurrentPeriodID)
			if err != nil || period.Status != "open" || period.UsedBytes != 77 {
				t.Fatal("activation reset paid usage", period, err)
			}
			foreign, err := f.store.XNodePeriod(ctx, other.CurrentPeriodID)
			if err != nil || foreign.Status != "open" || foreign.UsedBytes != 7 {
				t.Fatal("activation mutated foreign legacy target", foreign, err)
			}
			phase, err := f.store.PurchaseTrafficResetPhase(ctx, f.purchase.ID)
			if err != nil || phase != "pending" {
				t.Fatal("native activation used legacy reset phase", phase, err)
			}
			purchase, err := f.store.PurchaseByID(ctx, f.purchase.ID)
			if err != nil || purchase.Status != "active" || !purchase.UpdatedAt.Equal(f.purchase.UpdatedAt) {
				t.Fatal("replay changed settled activation", purchase, err)
			}
		})
	}
}

func TestNativeActivationProvisionsFirstOwnedPeriodAndCompletesExactPurchase(t *testing.T) {
	for _, retainedIdentity := range []bool{false, true} {
		t.Run(map[bool]string{false: "new identity", true: "purged identity"}[retainedIdentity], func(t *testing.T) {
			t.Parallel()
			f, ctx, now := newNativePurchaseFixture(t), context.Background(), time.Now().UTC()
			user, _, err := f.store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: f.user.TelegramID + 2}, false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete' WHERE id=$1`, user.ID); err != nil {
				t.Fatal(err)
			}
			if _, err = f.store.AdjustBalance(ctx, user.ID, 1000, "activation-first", "fixture", now); err != nil {
				t.Fatal(err)
			}
			purchase, err := f.store.CreatePurchase(ctx, database.PurchaseInput{UserID: user.ID, ComboID: f.purchase.ComboID, IdempotencyKey: "activation-first"}, now)
			if err != nil {
				t.Fatal(err)
			}
			if retainedIdentity {
				if _, err = f.store.DB().ExecContext(ctx, `UPDATE users SET account_generation=1 WHERE id=$1`, user.ID); err != nil {
					t.Fatal(err)
				}
				if _, err = f.store.SaveXNodeIdentity(ctx, xnode.Identity{UserID: user.ID, AccessDisabled: true}); err != nil {
					t.Fatal(err)
				}
			}
			payload, err := json.Marshal(map[string]string{"purchaseId": purchase.ID})
			if err != nil {
				t.Fatal(err)
			}
			worker := entitlements.NewWorker(f.store, f.provider, f.provider)
			job := model.OutboxJob{Kind: "xnode_apply_entitlement", Payload: string(payload)}
			for range 2 {
				if err = worker.HandleOutbox(ctx, job); err != nil {
					t.Fatal(err)
				}
			}
			identity, err := f.provider.native.IdentityForUser(ctx, user.ID)
			if err != nil || identity.UserID != user.ID || identity.CurrentPeriodID == "" {
				t.Fatal(identity, err)
			}
			period, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
			if err != nil || period.PurchaseID != purchase.ID || period.Status != "open" || period.LimitBytes != purchase.TrafficLimitBytes {
				t.Fatal(period, err)
			}
			completed, err := f.store.PurchaseByID(ctx, purchase.ID)
			if err != nil || completed.Status != "active" {
				t.Fatal("native activation was not completed", completed, err)
			}
			var count int
			if err = f.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_periods WHERE proxy_user_id=$1`, identity.ProxyUserID).Scan(&count); err != nil || count != 1 {
				t.Fatal("replay created additional quota", count, err)
			}
		})
	}
}
