package app

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestNativeAdminEditAndRefundResolvePanelOwner(t *testing.T) {
	for _, staleLink := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing legacy link", true: "another native account in legacy link"}[staleLink], func(t *testing.T) {
			f := newNativePurchaseFixture(t)
			ctx := context.Background()
			other, _, err := f.store.UpsertTelegramUser(ctx, model.TelegramProfile{ID: f.user.TelegramID + 1}, false)
			if err != nil {
				t.Fatal(err)
			}
			otherIdentity, err := f.provider.native.EnsureIdentity(ctx, other)
			if err != nil {
				t.Fatal(err)
			}
			if staleLink {
				if _, err := f.store.DB().ExecContext(ctx, `UPDATE users SET proxy_user_id=$1 WHERE id=$2`, strconv.FormatInt(otherIdentity.ProxyUserID, 10), f.user.ID); err != nil {
					t.Fatal(err)
				}
			}
			identity, err := f.provider.native.IdentityForUser(ctx, f.user.ID)
			if err != nil {
				t.Fatal(err)
			}
			periodID := identity.CurrentPeriodID
			if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=7 WHERE id=$1`, periodID); err != nil {
				t.Fatal(err)
			}
			workflows := admin.NewUserWorkflows(f.store, f.provider)
			_, err = workflows.EditEntitlement(ctx, f.user.ID, f.user.ID, f.purchase.ID, "edit-native", admin.EntitlementEdit{
				Version: f.purchase.UpdatedAt, Reason: "native allowance correction", ComboID: f.purchase.ComboID,
				ValidFrom: f.purchase.ValidFrom, ValidUntil: f.purchase.ValidUntil.Add(time.Hour), Status: "active",
				TrafficLimitBytes: 2000, ResetStrategy: f.purchase.ResetStrategy,
			})
			if err != nil {
				t.Fatal(err)
			}
			operations, err := f.store.ListAdminOperationsForUser(ctx, f.user.ID, 10)
			if err != nil || len(operations) != 1 {
				t.Fatalf("edit operation = %+v, %v", operations, err)
			}
			runNativeAdminOperation(t, f, operations[0].ID)
			period, err := f.store.XNodePeriod(ctx, periodID)
			if err != nil || period.LimitBytes != 2000 || period.UsedBytes != 7 || period.Status != "open" {
				t.Fatalf("owner period after edit = %+v, %v", period, err)
			}
			if err := f.provider.SyncAdminUserEntitlement(ctx, f.user, nil); err != nil {
				t.Fatalf("refresh empty stale snapshot: %v", err)
			}
			period, err = f.store.XNodePeriod(ctx, periodID)
			if err != nil || period.Status != "open" || period.UsedBytes != 7 {
				t.Fatalf("empty stale snapshot revoked paid access = %+v, %v", period, err)
			}
			receipt, err := workflows.RefundEntitlement(ctx, f.user.ID, f.user.ID, f.purchase.ID, "refund-native", "administrator refund", f.purchase.PriceTXBMinor)
			if err != nil {
				t.Fatal(err)
			}
			runNativeAdminOperation(t, f, receipt.ID)
			period, err = f.store.XNodePeriod(ctx, periodID)
			if err != nil || period.Status == "open" || period.UsedBytes != 7 {
				t.Fatalf("refund failed to close exact accounting = %+v, %v", period, err)
			}
			otherAfter, err := f.provider.native.IdentityForUser(ctx, other.ID)
			if err != nil || otherAfter.CurrentPeriodID != otherIdentity.CurrentPeriodID || otherAfter.AccessDisabled != otherIdentity.AccessDisabled {
				t.Fatalf("another account was changed = %+v, %v", otherAfter, err)
			}
		})
	}
}

func runNativeAdminOperation(t *testing.T, f nativePurchaseFixture, id string) {
	t.Helper()
	ctx := context.Background()
	operation, err := f.store.ProviderOperationByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	worker := admin.NewUserOperationWorker(f.store, f.provider)
	if err := worker.HandleProviderOperation(ctx, operation, model.OutboxJob{Attempts: 1}); err != nil {
		t.Fatal(err)
	}
	operation, err = f.store.ProviderOperationByID(ctx, id)
	if err != nil || operation.Receipt.Status != "succeeded" {
		t.Fatalf("native admin operation = %+v, %v", operation, err)
	}
}
