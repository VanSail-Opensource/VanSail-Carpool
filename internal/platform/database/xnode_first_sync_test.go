package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"testing"
	"time"
)

func TestFirstSyncIsOwnerScopedOnceAndSurvivesIdentityUpdates(t *testing.T) {
	s, identity, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	otherUser := createTestUser(t, s, 80402)
	other, err := s.SaveXNodeIdentity(ctx, xnode.Identity{UserID: otherUser.ID, CredentialsCipher: "encrypted"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err = s.ConfirmXNodeFirstSync(ctx, "node", 1, []int64{identity.ProxyUserID}, now); err != nil {
		t.Fatal(err)
	}
	before, err := s.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || before.FirstSyncedAt != nil {
		t.Fatal("node-wide state fabricated first sync", before, err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE xnode_nodes SET desired_revision=1,applied_revision=1 WHERE id='node'`); err != nil {
		t.Fatal(err)
	}
	if err = s.ConfirmXNodeFirstSync(ctx, "node", 1, []int64{identity.ProxyUserID}, now); err != nil {
		t.Fatal(err)
	}
	if err = s.ConfirmXNodeFirstSync(ctx, "node", 1, []int64{identity.ProxyUserID}, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	after, err := s.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || after.FirstSyncedAt == nil || !after.FirstSyncedAt.Equal(now) {
		t.Fatal(after, err)
	}
	foreign, err := s.XNodeIdentity(ctx, other.ProxyUserID)
	if err != nil || foreign.FirstSyncedAt != nil {
		t.Fatal("foreign identity confirmed", foreign, err)
	}
	if _, err = s.SaveXNodeIdentity(ctx, identity); err != nil {
		t.Fatal(err)
	}
	restarted := NewStore(s.db)
	persisted, err := restarted.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || persisted.FirstSyncedAt == nil || !persisted.FirstSyncedAt.Equal(now) {
		t.Fatal("identity update/restart erased proof", persisted, err)
	}
}
