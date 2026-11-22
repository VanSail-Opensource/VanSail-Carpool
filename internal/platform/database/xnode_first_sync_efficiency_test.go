package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestFirstSyncAlreadyProvenOwnerDoesNotReacquireMaterialLocks(t *testing.T) {
	s, parent, _, payg := paygBillingFixture(t)
	ctx, now := context.Background(), time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `UPDATE xnode_nodes SET desired_revision=1,applied_revision=1 WHERE id='node'`); err != nil {
		t.Fatal(err)
	}
	subject := capturedFirstSyncSubject(t, s, payg.ProxyUserID)
	if err := s.ConfirmXNodeFirstSync(ctx, "node", 1, []xnode.FirstSyncSubject{subject}, now); err != nil {
		t.Fatal(err)
	}
	owner, err := s.XNodeIdentity(ctx, parent.ProxyUserID)
	if err != nil || owner.FirstSyncedAt == nil {
		t.Fatal("fixture did not confirm its owner", owner, err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	var locked int64
	if err := tx.QueryRowContext(ctx, `SELECT proxy_user_id FROM xnode_identities WHERE proxy_user_id=$1 FOR UPDATE`, parent.ProxyUserID).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	var user string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, parent.UserID).Scan(&user); err != nil {
		t.Fatal(err)
	}
	// A repeated lease ACK has no readiness mutation to perform. It must return
	// while the material locks remain held, without weakening the existing
	// rotation/concurrent-replacement regressions for unproven owners.
	call, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.ConfirmXNodeFirstSync(call, "node", 1, []xnode.FirstSyncSubject{subject}, now.Add(time.Minute)); err != nil {
		t.Fatal("redundant first-sync proof waited on account/material locks", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	owner, err = s.XNodeIdentity(ctx, parent.ProxyUserID)
	if err != nil || owner.FirstSyncedAt == nil || !owner.FirstSyncedAt.Equal(now) {
		t.Fatal("redundant proof changed the original readiness fact", owner, err)
	}
}
