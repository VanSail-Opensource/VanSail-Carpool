package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestRoutingAccessPruneErasesOperationalHistoryAndFencesStaleWrites(t *testing.T) {
	store, identity, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	input := providerops.CreateInput{ActorUserID: identity.UserID, OwnerUserID: identity.UserID, Kind: xnode.RoutingOperationKind, IdempotencyKey: "access-prune", RequestFingerprint: "routing-access-prune-fingerprint", Items: []providerops.ItemInput{{Key: "routing", TargetType: "xnode_user", TargetID: fmt.Sprint(identity.ProxyUserID)}}}
	if _, err := store.QueueXNodeRouting(ctx, xnode.RoutingPlan{ProxyUserID: identity.ProxyUserID, Cipher: "old-choices", LegacyCipher: "immutable-evidence"}, input, now); err != nil {
		t.Fatal(err)
	}
	plan, err := store.XNodeRoutingPlan(ctx, identity.ProxyUserID)
	if err != nil {
		t.Fatal(err)
	}
	before, _, err := store.XNodeCompileVersion(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	plan.Cipher = "remaining-choices"
	if err := store.PruneXNodeRoutingAccess(ctx, plan, "another-user", before, now); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("owner fence lost", err)
	}
	if err := store.PruneXNodeRoutingAccess(ctx, plan, identity.UserID, before, now); err != nil {
		t.Fatal(err)
	}
	saved, err := store.XNodeRoutingPlan(ctx, identity.ProxyUserID)
	if err != nil || saved.Cipher != "remaining-choices" || saved.PreviousCipher != "" || saved.LegacyCipher != "immutable-evidence" || saved.Phase != "disabled" || saved.Reason != "routing_access_removed" || saved.Revision != plan.Revision+1 {
		t.Fatal(saved, err)
	}
	if err := store.PruneXNodeRoutingAccess(ctx, plan, identity.UserID, before, now); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("stale cleanup replaced newer graph", err)
	}
	if err := store.AdvanceXNodeRouting(ctx, identity.ProxyUserID, plan.OperationID, plan.Phase, "applying", now); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("old operation resurrected routes", err)
	}
	after, _, err := store.XNodeCompileVersion(ctx, now)
	if err != nil || after <= before {
		t.Fatal("node configurations were not invalidated", err)
	}
	if _, err := store.DB().Exec(`UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1`); err != nil {
		t.Fatal(err)
	}
	saved.Cipher = "obsolete-access-read"
	if err := store.PruneXNodeRoutingAccess(ctx, saved, identity.UserID, after, now); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("stale entitlement inventory pruned newer access", err)
	}

}
