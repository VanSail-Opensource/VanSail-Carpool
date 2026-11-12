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

func TestNativeEntitlementSyncCannotFollowAnotherOwnersLegacyLink(t *testing.T) {
	for _, paid := range []bool{false, true} {
		for _, stale := range []bool{false, true} {
			t.Run(map[bool]string{false: "ended", true: "paid"}[paid]+map[bool]string{false: "/missing-link", true: "/stale-link"}[stale], func(t *testing.T) {
				f := newNativePurchaseFixture(t)
				ctx := context.Background()
				other := anotherNativeSyncOwner(t, f)
				identity, err := f.provider.native.IdentityForUser(ctx, f.user.ID)
				if err != nil {
					t.Fatal(err)
				}
				if !paid {
					if _, err := f.store.DB().ExecContext(ctx, `UPDATE purchases SET status='cancelled' WHERE id=$1`, f.purchase.ID); err != nil {
						t.Fatal(err)
					}
				}
				if stale {
					if _, err := f.store.DB().ExecContext(ctx, `UPDATE users SET proxy_user_id=$1 WHERE id=$2`, strconv.FormatInt(other.ProxyUserID, 10), f.user.ID); err != nil {
						t.Fatal(err)
					}
				}
				payload, err := json.Marshal(map[string]string{"userId": f.user.ID})
				if err != nil {
					t.Fatal(err)
				}
				worker := entitlements.NewWorker(f.store, f.provider, f.provider)
				job := model.OutboxJob{Kind: "xnode_sync_user", Payload: string(payload)}
				for range 2 {
					if err := worker.HandleOutbox(ctx, job); err != nil {
						t.Fatal(err)
					}
				}
				ownPeriod, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
				if err != nil || (ownPeriod.Status == "open") != paid {
					t.Fatalf("owner desired access not synchronized: %+v %v", ownPeriod, err)
				}
				otherPeriod, err := f.store.XNodePeriod(ctx, other.CurrentPeriodID)
				if err != nil || otherPeriod.Status != "open" || otherPeriod.UsedBytes != 7 {
					t.Fatalf("another owner's accounting was changed: %+v %v", otherPeriod, err)
				}
			})
		}
	}
}

func anotherNativeSyncOwner(t *testing.T, f nativePurchaseFixture) xnode.Identity {
	t.Helper()
	ctx, now := context.Background(), time.Now().UTC()
	user, _, err := f.store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: f.user.TelegramID + 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	user.OnboardingState = "complete"
	if _, err := f.store.AdjustBalance(ctx, user.ID, 1000, "sync-other-seed", "fixture", now); err != nil {
		t.Fatal(err)
	}
	purchase, err := f.store.CreatePurchase(ctx, database.PurchaseInput{UserID: user.ID, ComboID: f.purchase.ComboID, IdempotencyKey: "sync-other"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.MarkPurchaseSyncResult(ctx, purchase.ID, true, now); err != nil {
		t.Fatal(err)
	}
	identity, err := f.provider.native.EnsureIdentity(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.provider.native.ApplyPurchase(ctx, strconv.FormatInt(identity.ProxyUserID, 10), purchase.TrafficLimitBytes, purchase.ResetStrategy, purchase.ValidUntil); err != nil {
		t.Fatal(err)
	}
	identity, err = f.provider.native.IdentityForUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=7 WHERE id=$1`, identity.CurrentPeriodID); err != nil {
		t.Fatal(err)
	}
	return identity
}
