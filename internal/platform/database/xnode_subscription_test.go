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

func TestMemberSubscriptionRotationRejectsStaleAccountGeneration(t *testing.T) {
	s, identity, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	expected, err := s.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || expected.UserID == "" || expected.ServiceName != "" {
		t.Fatal("fixture is not a direct member identity", expected, err)
	}
	var generation int64
	if err := s.db.QueryRowContext(ctx, `SELECT account_generation FROM users WHERE id=$1`, expected.UserID).Scan(&generation); err != nil {
		t.Fatal(err)
	}
	// Lifecycle replacement may retain the same native material. The old
	// authenticated request must still fail, even when its material CAS matches.
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET account_generation=account_generation+1 WHERE id=$1`, expected.UserID); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte("member-rotation"))
	if _, err := s.RotateXNodeMemberSubscription(ctx, expected, generation, hash[:], "stale-generation"); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("old generation rotated unchanged native material", err)
	}
	after, err := s.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || after.SubscriptionCipher != expected.SubscriptionCipher {
		t.Fatal("rejected member rotation changed native material", after, err)
	}
	current, err := s.RotateXNodeMemberSubscription(ctx, expected, generation+1, hash[:], "current-generation")
	if err != nil || current.SubscriptionCipher != "current-generation" || current.CurrentPeriodID != expected.CurrentPeriodID || current.CredentialsCipher != expected.CredentialsCipher {
		t.Fatal("current member rotation did not preserve native state", current, err)
	}
	if _, err := s.RotateXNodeMemberSubscription(ctx, expected, generation+1, hash[:], "stale-material"); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("member rotation accepted obsolete material", err)
	}
	current.ServiceName = "service-identity"
	if _, err := s.RotateXNodeMemberSubscription(ctx, current, generation+1, hash[:], "service-rotation"); !errors.Is(err, xnode.ErrInvalid) {
		t.Fatal("member rotation accepted a service identity", err)
	}
}
