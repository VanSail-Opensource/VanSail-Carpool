package app

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
	"testing"
	"time"
)

func TestUnpaidResetReceiptCannotMutateNativePeriods(t *testing.T) {
	f := newNativePurchaseFixture(t)
	ctx := context.Background()
	operation, _, err := f.store.CreateProviderOperation(ctx, providerops.CreateInput{ActorUserID: f.user.ID, OwnerUserID: f.user.ID, Kind: "purchase_traffic_reset", IdempotencyKey: "unpaid-reset", RequestFingerprint: "retired-reset-admission", Items: []providerops.ItemInput{{Key: "purchase", TargetType: "purchase", TargetID: f.purchase.ID}}}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	identity, err := f.provider.native.IdentityForUser(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	before, err := f.store.XNodePeriods(ctx, identity.ProxyUserID)
	if err != nil {
		t.Fatal(err)
	}
	if err = purchaseops.NewWorker(f.store, f.provider).HandleProviderOperation(ctx, operation, model.OutboxJob{Attempts: 1}); err != nil {
		t.Fatal(err)
	}
	after, err := f.store.XNodePeriods(ctx, identity.ProxyUserID)
	if err != nil || len(after) != len(before) {
		t.Fatal("unpaid reset changed periods", before, after, err)
	}
	result, err := f.store.ProviderOperationByID(ctx, operation.Receipt.ID)
	if err != nil || result.Receipt.Status != "failed" {
		t.Fatal(result, err)
	}
}
