package database

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestHistoricalPaymentRejectionKeepsEvidencePrivate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newTestStore(t)
	actor := createTestUser(t, store, 98_884)
	owner := createTestUser(t, store, 98_885)
	now := time.Now().UTC()
	input := providerops.CreateInput{
		ActorUserID: actor.ID, OwnerUserID: owner.ID, Kind: "payment_create",
		IdempotencyKey: "historical-rejection", RequestFingerprint: "historical-rejection-fingerprint",
		Items: []providerops.ItemInput{{Key: "payment", TargetType: "payment_order", TargetID: "historical-order"}},
	}
	operation, _, err := store.CreateProviderOperation(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	const historical = `{"paymentOrderId":"historical-order","providerMessage":"notify_url=https://app.example/webhooks/bepusdt/private-callback-capability signature=signed-request-secret"}`
	completed, err := store.CompleteProviderOperation(ctx, operation.Receipt.ID, providerops.Completion{
		Status: providerops.StatusFailed, ErrorCode: "PAYMENT_CREATE_REJECTED", ResultJSON: historical,
	}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	recorded := completed.ResultJSON
	if !strings.Contains(recorded, "private-callback-capability") {
		t.Fatal("historical evidence fixture did not retain the payload")
	}
	receipts := []model.OperationReceipt{completed.Receipt}
	receipt, err := store.ProviderOperationForOwner(ctx, operation.Receipt.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	receipts = append(receipts, receipt)
	for _, userID := range []string{actor.ID, owner.ID} {
		receipt, err := store.ProviderOperationForPrincipal(ctx, operation.Receipt.ID, userID)
		if err != nil {
			t.Fatal(err)
		}
		receipts = append(receipts, receipt)
	}
	listed, err := store.ListProviderOperationsForOwner(ctx, owner.ID, 10)
	if err != nil || len(listed) != 1 {
		t.Fatalf("owner receipt inventory = %+v, %v", listed, err)
	}
	receipts = append(receipts, listed...)
	replayed, replay, err := store.CreateOrReplayProviderOperation(ctx, input, now.Add(time.Minute))
	if err != nil || !replay || replayed.ResultJSON != recorded {
		t.Fatalf("historical replay changed: %+v, %t, %v", replayed, replay, err)
	}
	receipts = append(receipts, replayed.Receipt)
	for _, receipt := range receipts {
		if receipt.ErrorMessage == nil || *receipt.ErrorMessage != "Payment provider rejected this checkout request." ||
			receipt.ErrorCode == nil || *receipt.ErrorCode != "PAYMENT_CREATE_REJECTED" {
			t.Fatalf("public rejection projection = %+v", receipt)
		}
		encoded, err := json.Marshal(receipt)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "private-callback-capability") || strings.Contains(string(encoded), "signed-request-secret") {
			t.Fatal("historical provider payload escaped through a public receipt")
		}
	}
	retained, err := store.ProviderOperationByID(ctx, operation.Receipt.ID)
	if err != nil || retained.ResultJSON != recorded || retained.Receipt.UpdatedAt != completed.Receipt.UpdatedAt {
		t.Fatalf("historical evidence changed: %+v, %v", retained, err)
	}
}
