package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPAYGReopenCannotRestoreCredentialsAfterANewerPurge(t *testing.T) {
	s, parent, _, account := paygBillingFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := s.db.ExecContext(ctx, `UPDATE users SET account_generation=1 WHERE id=$1`, parent.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE xnode_identities SET credentials_cipher='',access_disabled=1 WHERE proxy_user_id=$1`, account.ProxyUserID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE xnode_periods SET status='closed',ended_at=$2 WHERE id=$1`, account.PeriodID, stamp(time.Now().UTC())); err != nil {
		t.Fatal(err)
	}
	identity, err := s.XNodeIdentity(ctx, account.ProxyUserID)
	if err != nil {
		t.Fatal(err)
	}
	identity.CredentialsCipher = "new-join-private-secret"
	period := xnode.Period{ID: "payg-late-rejoin", ProxyUserID: identity.ProxyUserID, StartedAt: time.Now().UTC()}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, parent.UserID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.ReopenPurgedPAYG(ctx, identity, period, parent.UserID, 1, 100) }()
	waitNativeIdentityQueryBlocked(t, s, ctx, "FOR UPDATE")
	if _, err = tx.ExecContext(ctx, `UPDATE users SET account_status='purged',account_generation=2 WHERE id=$1`, parent.UserID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-done; !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("stale rejoin crossed final purge", err)
	}
	stored, err := s.XNodeIdentity(ctx, identity.ProxyUserID)
	if err != nil || stored.CredentialsCipher != "" || !stored.AccessDisabled {
		t.Fatal("stale rejoin restored private credential", stored, err)
	}
	if _, err = s.XNodePeriod(ctx, period.ID); !errors.Is(err, xnode.ErrNotFound) {
		t.Fatal("stale rejoin created an unowned access period", err)
	}
}
