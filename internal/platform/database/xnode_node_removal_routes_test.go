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

func removalPlanFixture(t *testing.T, s *Store, owner xnode.Identity) xnode.RoutingPlan {
	t.Helper()
	input := providerops.CreateInput{ActorUserID: owner.UserID, OwnerUserID: owner.UserID, Kind: xnode.RoutingOperationKind, IdempotencyKey: "node-removal-routing", RequestFingerprint: "node-removal-routing-fingerprint", Items: []providerops.ItemInput{{Key: "routing", TargetType: "xnode_user", TargetID: fmt.Sprint(owner.ProxyUserID)}}}
	plan := xnode.RoutingPlan{ProxyUserID: owner.ProxyUserID, Cipher: "saved-routing", LegacyCipher: "immutable-history"}
	if _, err := s.QueueXNodeRouting(context.Background(), plan, input, time.Now()); err != nil {
		t.Fatal(err)
	}
	saved, err := s.XNodeRoutingPlan(context.Background(), owner.ProxyUserID)
	if err != nil {
		t.Fatal(err)
	}
	return saved
}

func TestNodeRemovalKeepsRevocationProofUntilDrained(t *testing.T) {
	s, owner, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	plan := removalPlanFixture(t, s, owner)
	plan.Cipher, plan.PreviousCipher = "pruned-routing", "revocation-proof"
	version, _, err := s.XNodeCompileVersion(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.PrepareXNodeRemoval(ctx, "node", version, plan); err != nil {
		t.Fatal(err)
	}
	pending, err := s.XNodeRoutingPlan(ctx, owner.ProxyUserID)
	if err != nil || pending.Phase != "disabled" || !xnode.RoutingRemovalPending(pending) || pending.PreviousCipher != "revocation-proof" || pending.LegacyCipher != "immutable-history" {
		t.Fatalf("lost pending proof: %+v %v", pending, err)
	}
	input := providerops.CreateInput{ActorUserID: owner.UserID, OwnerUserID: owner.UserID, Kind: xnode.RoutingOperationKind, IdempotencyKey: "resave-during-removal", RequestFingerprint: "resave-during-removal-fingerprint", Items: []providerops.ItemInput{{Key: "routing", TargetType: "xnode_user", TargetID: fmt.Sprint(owner.ProxyUserID)}}}
	if _, err = s.QueueXNodeRouting(ctx, pending, input, time.Now()); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("pending graph overwritten: %v", err)
	}
	version, _, err = s.XNodeCompileVersion(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.FinishXNodeRemoval(ctx, "node", version, owner.UserID, "drained"); err != nil {
		t.Fatal(err)
	}
	done, err := s.XNodeRoutingPlan(ctx, owner.ProxyUserID)
	if err != nil || done.PreviousCipher != "" || done.Reason != "routing_node_removed" || done.Phase != "disabled" || done.Cipher != pending.Cipher || done.LegacyCipher != pending.LegacyCipher {
		t.Fatalf("incorrect final graph: %+v %v", done, err)
	}
}

func TestStaleNodeRemovalPlanRollsBackAdmissionFence(t *testing.T) {
	s, owner, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	plan := removalPlanFixture(t, s, owner)
	plan.Revision++
	version, _, err := s.XNodeCompileVersion(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.PrepareXNodeRemoval(ctx, "node", version, plan); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("stale plan admitted: %v", err)
	}
	node, err := s.XNodeNode(ctx, "node")
	if err != nil || node.RemovingAt != nil || !node.Enabled {
		t.Fatalf("failed prune changed admission: %+v %v", node, err)
	}
}

func TestWaitingNodeRemovalKeepsMonotonicReceiptAttempts(t *testing.T) {
	s, _, _ := xnodeTrafficFixture(t)
	actor := createTestAdmin(t, s, 80493)
	ctx := context.Background()
	now := time.Now()
	input := providerops.CreateInput{ActorUserID: actor.ID, Kind: xnode.OperationKind, IdempotencyKey: "waiting-node-removal", RequestFingerprint: "waiting-node-removal-fingerprint", Items: []providerops.ItemInput{{Key: "node", TargetType: "xnode", TargetID: "node"}}}
	operation, _, err := s.CreateProviderOperation(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.WaitXNodeRemoval(ctx, operation.Receipt.ID, first.Attempts, "XNODE_REMOVAL_OFFLINE", now); err != nil {
		t.Fatal(err)
	}
	queued, err := s.ProviderOperationByID(ctx, operation.Receipt.ID)
	if err != nil || queued.Receipt.Status != "queued" || queued.Receipt.ErrorCode == nil || *queued.Receipt.ErrorCode != "XNODE_REMOVAL_OFFLINE" {
		t.Fatalf("dependency lost: %+v %v", queued, err)
	}
	next, err := s.BeginProviderOperationAttempt(ctx, operation.Receipt.ID, now)
	if err != nil || next.Attempts <= first.Attempts {
		t.Fatalf("attempt fence reused: %+v %v", next, err)
	}
	if _, err = s.CompleteXNodeProviderOperation(ctx, operation.Receipt.ID, first.Attempts, providerops.Completion{Status: providerops.StatusSucceeded}, now); err == nil {
		t.Fatal("old callback completed recovered removal")
	}
}
