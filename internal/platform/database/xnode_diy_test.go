package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestDIYAtomicRevisionsReplayAndUnchangedRouting(t *testing.T) {
	store, identity, _ := xnodeTrafficFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	doc := xnode.DIYDocument{UserID: identity.UserID, Cipher: "encrypted-client-only"}
	input := providerops.CreateInput{ActorUserID: identity.UserID, OwnerUserID: identity.UserID, Kind: xnode.DIYOperationKind, IdempotencyKey: "settings-one", RequestFingerprint: strings.Repeat("f", 64), Items: []providerops.ItemInput{{Key: "settings", TargetType: "member_diy", TargetID: identity.UserID}}}
	receipt, err := store.SaveXNodeDIY(ctx, doc, 0, nil, input, now)
	if err != nil || receipt.Status != string(providerops.StatusSucceeded) {
		t.Fatal("local save", receipt, err)
	}
	saved, err := store.XNodeDIY(ctx, identity.UserID)
	if err != nil || saved.Revision != 1 || saved.Cipher != doc.Cipher {
		t.Fatal("encrypted document", saved, err)
	}
	var routeCount int
	if err = store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_routing_plans WHERE proxy_user_id=?`, identity.ProxyUserID).Scan(&routeCount); err != nil || routeCount != 0 {
		t.Fatal("settings-only save redeployed routes", err)
	}
	if replay, found, routed, err := store.ReplayXNodeDIY(ctx, identity.UserID, input.IdempotencyKey, input.RequestFingerprint); err != nil || !found || routed || replay.ID != receipt.ID {
		t.Fatal("idempotent settings receipt", replay, found, routed, err)
	}
	if _, _, _, err = store.ReplayXNodeDIY(ctx, identity.UserID, input.IdempotencyKey, "changed"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("changed body replay accepted", err)
	}
	if _, err = store.ProviderOperationForOwner(ctx, receipt.ID, "another-owner"); err == nil {
		t.Fatal("owner isolation")
	}
	input.IdempotencyKey = "combined"
	input.Items = []providerops.ItemInput{{Key: "routing", TargetType: "xnode_user", TargetID: fmt.Sprint(identity.ProxyUserID)}}
	plan := &xnode.RoutingPlan{ProxyUserID: identity.ProxyUserID, Cipher: "encrypted-route"}
	if _, err = store.SaveXNodeDIY(ctx, doc, 0, plan, input, now); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("stale settings revision", err)
	}
	if _, found, _, err := store.ReplayXNodeDIY(ctx, identity.UserID, input.IdempotencyKey, input.RequestFingerprint); err != nil || found {
		t.Fatal("failed atomic save leaked receipt", found, err)
	}
	doc.Revision = 1
	if _, err = store.SaveXNodeDIY(ctx, doc, 7, plan, input, now); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("stale routing revision", err)
	}
	combined, err := store.SaveXNodeDIY(ctx, doc, 0, plan, input, now)
	if err != nil || combined.Status != string(providerops.StatusQueued) {
		t.Fatal("combined save", combined, err)
	}
	saved, err = store.XNodeDIY(ctx, identity.UserID)
	if err != nil || saved.Revision != 2 {
		t.Fatal(saved, err)
	}
	route, err := store.XNodeRoutingPlan(ctx, identity.ProxyUserID)
	if err != nil || route.Revision != 1 || route.OperationID != combined.ID || route.Phase != "revoking" {
		t.Fatal("routing state not atomic", route, err)
	}
	// Subscription-only changes remain local while routing deploys.
	doc.Revision = 2
	input.IdempotencyKey = "settings-during-deployment"
	input.Items = []providerops.ItemInput{{Key: "settings", TargetType: "member_diy", TargetID: identity.UserID}}
	local, err := store.SaveXNodeDIY(ctx, doc, 1, nil, input, now)
	if err != nil || local.Status != string(providerops.StatusSucceeded) {
		t.Fatal("local save during deployment", err)
	}
	after, err := store.XNodeRoutingPlan(ctx, identity.ProxyUserID)
	if err != nil || after.OperationID != combined.ID || after.Revision != route.Revision || after.Cipher != route.Cipher {
		t.Fatal("unchanged routing mutated", after, err)
	}
	if _, err = store.SaveXNodeDIY(ctx, xnode.DIYDocument{UserID: "another-owner", Cipher: "wrong"}, 0, nil, input, now); err == nil {
		t.Fatal("cross-owner save accepted")
	}
}
