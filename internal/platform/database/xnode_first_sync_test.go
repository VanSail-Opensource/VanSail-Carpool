package database

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func capturedFirstSyncSubject(t *testing.T, s *Store, principal int64) xnode.FirstSyncSubject {
	t.Helper()
	ctx := context.Background()
	identity, err := s.XNodeIdentity(ctx, principal)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := s.XNodeIdentityByUser(ctx, identity.MemberOwnerID())
	if err != nil {
		t.Fatal(err)
	}
	account, err := s.UserByID(ctx, owner.UserID)
	if err != nil {
		t.Fatal(err)
	}
	return xnode.FirstSyncSubject{
		ProxyUserID:         principal,
		OwnerProxyUserID:    owner.ProxyUserID,
		OwnerID:             owner.UserID,
		AccountGeneration:   account.AccountGeneration,
		PeriodID:            identity.CurrentPeriodID,
		CredentialHash:      sha256.Sum256([]byte(identity.CredentialsCipher)),
		OwnerCredentialHash: sha256.Sum256([]byte(owner.CredentialsCipher)),
	}
}

func TestFirstSyncIsOwnerScopedOnceAndSurvivesIdentityUpdates(t *testing.T) {
	s, identity, _ := xnodeTrafficFixture(t)
	ctx := context.Background()
	otherUser := createTestUser(t, s, 80402)
	other, err := s.SaveXNodeIdentity(ctx, xnode.Identity{UserID: otherUser.ID, CredentialsCipher: "encrypted"})
	if err != nil {
		t.Fatal(err)
	}
	subject := capturedFirstSyncSubject(t, s, identity.ProxyUserID)
	now := time.Now().UTC()
	if err = s.ConfirmXNodeFirstSync(ctx, "node", 1, []xnode.FirstSyncSubject{subject}, now); err != nil {
		t.Fatal(err)
	}
	before, err := s.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || before.FirstSyncedAt != nil {
		t.Fatal("node-wide state fabricated first sync", before, err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE xnode_nodes SET desired_revision=1,applied_revision=1 WHERE id='node'`); err != nil {
		t.Fatal(err)
	}
	if err = s.ConfirmXNodeFirstSync(ctx, "node", 1, []xnode.FirstSyncSubject{subject}, now); err != nil {
		t.Fatal(err)
	}
	if err = s.ConfirmXNodeFirstSync(ctx, "node", 1, []xnode.FirstSyncSubject{subject}, now.Add(time.Hour)); err != nil {
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
	// Subscription-token rotation and ordinary identity saves do not replace
	// native credentials and must preserve genuine readiness through outages.
	after.SubscriptionCipher = "rotated-subscription"
	hash := sha256.Sum256([]byte("rotated-subscription-hash"))
	after.SubscriptionHash = hash[:]
	saved, err := s.SaveXNodeIdentity(ctx, after)
	if err != nil || saved.FirstSyncedAt == nil || !saved.FirstSyncedAt.Equal(now) {
		t.Fatal("token-only rotation erased proof", saved, err)
	}
	restarted := NewStore(s.db)
	persisted, err := restarted.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || persisted.FirstSyncedAt == nil || !persisted.FirstSyncedAt.Equal(now) {
		t.Fatal("identity update/restart erased proof", persisted, err)
	}
}

func TestFirstSyncCredentialRotationRejectsDelayedSamePeriodProof(t *testing.T) {
	s, identity, _ := xnodeTrafficFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `UPDATE xnode_nodes SET desired_revision=1,applied_revision=1 WHERE id='node'`); err != nil {
		t.Fatal(err)
	}
	old := capturedFirstSyncSubject(t, s, identity.ProxyUserID)
	if err := s.ConfirmXNodeFirstSync(ctx, "node", 1, []xnode.FirstSyncSubject{old}, now); err != nil {
		t.Fatal(err)
	}
	current, err := s.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || current.FirstSyncedAt == nil {
		t.Fatal("fixture did not confirm original material", current, err)
	}
	current.CredentialsCipher = "replacement-native-material"
	replacement, err := s.SaveXNodeIdentity(ctx, current)
	if err != nil || replacement.FirstSyncedAt != nil || replacement.CurrentPeriodID != old.PeriodID {
		t.Fatal("same-period material replacement retained proof", replacement, err)
	}
	if err = s.ConfirmXNodeFirstSync(ctx, "node", 1, []xnode.FirstSyncSubject{old}, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	unproven, err := s.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || unproven.FirstSyncedAt != nil {
		t.Fatal("delayed old material proved replacement credentials", unproven, err)
	}
	fresh := capturedFirstSyncSubject(t, s, identity.ProxyUserID)
	if err = s.ConfirmXNodeFirstSync(ctx, "node", 1, []xnode.FirstSyncSubject{fresh}, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	proven, err := s.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || proven.FirstSyncedAt == nil {
		t.Fatal("fresh applied material could not reconfirm", proven, err)
	}
}

func TestNewNativeIdentityCannotInjectFirstSyncProof(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "allocated_id", true: "explicit_id"}[explicit], func(t *testing.T) {
			s := newTestStore(t)
			user := createTestUser(t, s, 80403)
			at := time.Now().UTC()
			candidate := xnode.Identity{UserID: user.ID, CredentialsCipher: "new-material", FirstSyncedAt: &at}
			if explicit {
				candidate.ProxyUserID = user.TelegramID
			}
			saved, err := s.SaveXNodeIdentity(context.Background(), candidate)
			if err != nil || saved.FirstSyncedAt != nil {
				t.Fatal("identity initializer returned caller-supplied proof", saved, err)
			}
			stored, err := s.XNodeIdentity(context.Background(), saved.ProxyUserID)
			if err != nil || stored.FirstSyncedAt != nil {
				t.Fatal("identity initializer persisted caller-supplied proof", stored, err)
			}
		})
	}
}

func TestFirstSyncWaitsForConcurrentChildCredentialReplacement(t *testing.T) {
	s, parent, _, payg := paygBillingFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `UPDATE xnode_nodes SET desired_revision=1,applied_revision=1 WHERE id='node'`); err != nil {
		t.Fatal(err)
	}
	old := capturedFirstSyncSubject(t, s, payg.ProxyUserID)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_identities SET credentials_cipher='replacement-child-material' WHERE proxy_user_id=$1`, payg.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- s.ConfirmXNodeFirstSync(ctx, "node", 1, []xnode.FirstSyncSubject{old}, now)
	}()
	blocked := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database()
 AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND strpos(query,'ORDER BY proxy_user_id FOR UPDATE')>0)`).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("receipt did not wait for the captured principal's material lock")
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	owner, err := s.XNodeIdentity(ctx, parent.ProxyUserID)
	if err != nil || owner.FirstSyncedAt != nil {
		t.Fatal("concurrent child rotation accepted an obsolete receipt", owner, err)
	}
	fresh := capturedFirstSyncSubject(t, s, payg.ProxyUserID)
	if err = s.ConfirmXNodeFirstSync(ctx, "node", 1, []xnode.FirstSyncSubject{fresh}, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	owner, err = s.XNodeIdentity(ctx, parent.ProxyUserID)
	if err != nil || owner.FirstSyncedAt == nil {
		t.Fatal("fresh child receipt did not confirm its owner", owner, err)
	}
}
