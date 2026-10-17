package database

import (
	"context"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"testing"
	"time"
)

func TestRoutingPlanTransactionRetainsBarriersAndReplay(t *testing.T) {
	store, identity, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	at := time.Now().UTC()
	input := providerops.CreateInput{ActorUserID: identity.UserID, OwnerUserID: identity.UserID, Kind: xnode.RoutingOperationKind, IdempotencyKey: "routing-one", RequestFingerprint: "routing-fingerprint-one", Items: []providerops.ItemInput{{Key: "routing", TargetType: "xnode_user", TargetID: fmt.Sprint(identity.ProxyUserID)}}}
	p := xnode.RoutingPlan{ProxyUserID: identity.ProxyUserID, Cipher: "encrypted-one", LegacyCipher: "encrypted-evidence"}
	receipt, err := store.QueueXNodeRouting(ctx, p, input, at)
	if err != nil {
		t.Fatal(err)
	}
	ownerReceipt, err := store.ProviderOperationForOwner(ctx, receipt.ID, identity.UserID)
	if err != nil || ownerReceipt.ID != receipt.ID {
		t.Fatal("receipt owner lost", err)
	}
	if _, err = store.ProviderOperationForOwner(ctx, receipt.ID, "another-owner"); err == nil {
		t.Fatal("receipt leaked across owners")
	}
	if replay, err := store.QueueXNodeRouting(ctx, p, input, at); err != nil || replay.ID != receipt.ID {
		t.Fatal("request replay created another generation", err)
	}
	bad := input
	bad.RequestFingerprint = "different-fingerprint"
	if _, err = store.QueueXNodeRouting(ctx, p, bad, at); err == nil {
		t.Fatal("changed replay accepted")
	}
	input.IdempotencyKey = "routing-two"
	input.RequestFingerprint = "routing-fingerprint-two"
	p.Revision = 1
	p.Cipher = "encrypted-two"
	if _, err = store.QueueXNodeRouting(ctx, p, input, at); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("concurrent graph accepted %v", err)
	}
	for _, pair := range [][2]string{{"revoking", "applying"}, {"applying", "enabling"}, {"enabling", "active"}} {
		if err = store.AdvanceXNodeRouting(ctx, identity.ProxyUserID, receipt.ID, pair[0], pair[1], at); err != nil {
			t.Fatal(err)
		}
	}
	second, err := store.QueueXNodeRouting(ctx, p, input, at)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := store.XNodeRoutingPlan(ctx, identity.ProxyUserID)
	if err != nil || saved.PreviousCipher != "encrypted-one" || saved.LegacyCipher != "encrypted-evidence" || saved.Phase != "revoking" {
		t.Fatalf("old evidence lost: %+v %v", saved, err)
	}
	if err = store.AdvanceXNodeRouting(ctx, identity.ProxyUserID, receipt.ID, "revoking", "applying", at); err == nil {
		t.Fatal("old operation crossed new generation")
	}
	first, err := store.BeginProviderOperationAttempt(ctx, second.ID, at)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := store.BeginProviderOperationAttempt(ctx, second.ID, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CompleteXNodeProviderOperation(ctx, second.ID, first.Attempts, providerops.Completion{Status: providerops.StatusSucceeded}, at); err == nil {
		t.Fatal("stale worker completed recovered routing")
	}
	if recovered.Attempts <= first.Attempts {
		t.Fatal("recovery did not fence attempts")
	}
	events, err := store.ListAuditEvents(ctx, 20)
	if err != nil || len(events) < 5 {
		t.Fatal("routing audit evidence missing", err)
	}
}
