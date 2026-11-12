package database

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestSubscriptionRotationPreservesConcurrentPeriodAndReadiness(t *testing.T) {
	s, identity, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	proof := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `UPDATE xnode_identities SET first_synced_at=$1 WHERE proxy_user_id=$2`, stamp(proof), identity.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	expected, err := s.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || expected.CurrentPeriodID == "" {
		t.Fatal("fixture has no captured period", expected, err)
	}
	// A token-only write must not restore this stale accounting pointer.
	if _, err = s.db.ExecContext(ctx, `UPDATE xnode_identities SET current_period_id=NULL WHERE proxy_user_id=$1`, identity.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("rotated"))
	current, err := s.RotateXNodeSubscription(ctx, expected, hash[:], "rotated-cipher")
	if err != nil || current.CurrentPeriodID != "" || current.CredentialsCipher != expected.CredentialsCipher ||
		current.FirstSyncedAt == nil || !current.FirstSyncedAt.Equal(proof) {
		t.Fatal("token rotation replaced concurrent authority", current, err)
	}
	if current.SubscriptionCipher != "rotated-cipher" {
		t.Fatal("token was not rotated")
	}
	if _, err = s.RotateXNodeSubscription(ctx, expected, hash[:], "obsolete-replay"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("stale token rotation was accepted", err)
	}
	after, err := s.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || after.CurrentPeriodID != "" || after.SubscriptionCipher != "rotated-cipher" {
		t.Fatal("replay overwrote the successful rotation", after, err)
	}
}

func TestSubscriptionRotationRejectsReplacedMaterialAndForeignOwner(t *testing.T) {
	s, identity, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	expected, err := s.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("rotated"))
	foreign := expected
	foreign.UserID = ""
	foreign.ServiceName = "foreign"
	if _, err = s.RotateXNodeSubscription(ctx, foreign, hash[:], "foreign-cipher"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("foreign identity rotated the member token", err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE xnode_identities SET credentials_cipher='replacement-material',subscription_cipher='replacement-token' WHERE proxy_user_id=$1`, identity.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RotateXNodeSubscription(ctx, expected, hash[:], "stale-cipher"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("captured old material crossed replacement", err)
	}
	after, err := s.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || after.CredentialsCipher != "replacement-material" || after.SubscriptionCipher != "replacement-token" {
		t.Fatal("replacement credentials were restored from stale input", after, err)
	}
}
