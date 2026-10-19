package app

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

func TestNativeResetRecoveryRequiresItsOwnDurableProof(t *testing.T) {
	for _, mode := range []string{"operation", "natural cadence", "not attempted"} {
		t.Run(mode, func(t *testing.T) {
			f := newNativePurchaseFixture(t)
			ctx := context.Background()
			receipt := f.resetReceipt(t)
			operation, err := f.store.BeginProviderOperationAttempt(ctx, receipt.ID, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.store.BeginProviderOperationItemAttempt(ctx, receipt.ID, "purchase", time.Now()); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "operation":
				remote, id, err := f.provider.PurchaseRemote(ctx, f.user, f.purchase)
				if err != nil {
					t.Fatal(err)
				}
				if err := remote.(purchaseops.ResetOperationRemote).ResetTrafficForOperation(ctx, id, receipt.ID); err != nil {
					t.Fatal(err)
				}
			case "natural cadence":
				if err := f.provider.native.ResetTraffic(ctx, f.remoteID); err != nil {
					t.Fatal(err)
				}
			}
			identity, err := f.provider.native.IdentityForUser(ctx, f.user.ID)
			if err != nil {
				t.Fatal(err)
			}
			periods, err := f.store.XNodePeriods(ctx, identity.ProxyUserID)
			if err != nil {
				t.Fatal(err)
			}
			if err := purchaseops.NewWorker(f.store, f.provider).HandleProviderOperation(ctx, operation, model.OutboxJob{Attempts: 2}); err != nil {
				t.Fatal(err)
			}
			operation, err = f.store.ProviderOperationByID(ctx, receipt.ID)
			if err != nil {
				t.Fatal(err)
			}
			expected := "pending_review"
			if mode == "operation" {
				expected = "succeeded"
			}
			if operation.Receipt.Status != expected {
				t.Fatal("reset timestamp was mistaken for operation proof", mode, operation.Receipt)
			}
			after, err := f.store.XNodePeriods(ctx, identity.ProxyUserID)
			if err != nil || len(after) != len(periods) {
				t.Fatal("recovery repeated the reset", len(periods), len(after), err)
			}
		})
	}
}
