package database

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestNativeAdminPreservesLifecycleAndMaterialGuards(t *testing.T) {
	t.Parallel()
	ctx, f := context.Background(), newXNodeAdminFixture(t)
	if err := f.store.DeleteAdminXNodeNode(ctx, f.actor, f.node.ID); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("administrator bypassed unused-node deletion guard", err)
	}
	version, _, err := f.store.XNodeCompileVersion(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = f.store.DeleteAdminXNodeGroupAtVersion(ctx, f.actor, "delete-group", version-1, nil); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("administrator bypassed group compile revision", err)
	}
	hash := sha256.Sum256([]byte("rotated"))
	if _, err = f.store.RotateAdminXNodeSubscription(ctx, f.actor, f.identity, hash[:], "rotated-cipher"); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.RotateAdminXNodeSubscription(ctx, f.actor, f.identity, hash[:], "stale-replacement"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("administrator bypassed subscription material comparison", err)
	}
	if _, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_nodes SET enabled=0,removing_at=$1 WHERE id=$2`, stamp(time.Now()), f.node.ID); err != nil {
		t.Fatal(err)
	}
	before := xnodeAdminSnapshot(t, f.store)
	if err = f.store.RotateAdminXNodeToken(ctx, f.actor, f.node.ID, hash[:]); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("administrator enrolled a draining node", err)
	}
	if _, err = f.store.SaveAdminXNodeNodeDDNS(ctx, f.actor, f.node, nil); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("administrator rewrote a draining node", err)
	}
	if after := xnodeAdminSnapshot(t, f.store); after != before {
		t.Fatal("rejected draining-node edits changed durable state")
	}
}

func TestNativeAdminCustomizationRetainsMemberManagedBoundary(t *testing.T) {
	t.Parallel()
	ctx, f := context.Background(), newXNodeAdminFixture(t)
	member := createTestUser(t, f.store, 969002)
	identity, err := f.store.SaveXNodeIdentity(ctx, xnode.Identity{UserID: member.ID, CredentialsCipher: "member-credentials", SubscriptionCipher: "member-subscription"})
	if err != nil {
		t.Fatal(err)
	}
	before := xnodeAdminSnapshot(t, f.store)
	if _, err = f.store.SaveAdminXNodeCustomization(ctx, f.actor, xnode.Customization{ProxyUserID: identity.ProxyUserID, NodeID: f.node.ID, OutboundsCipher: "raw", RoutingCipher: "raw"}); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("named-service routing wrote a member's managed scope", err)
	}
	if after := xnodeAdminSnapshot(t, f.store); after != before {
		t.Fatal("rejected raw member routing changed durable state")
	}
}
