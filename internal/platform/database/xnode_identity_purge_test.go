package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestIdentityWriteWaitsForPurgeAndCannotRestoreCredentials(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "new_identity", true: "existing_identity"}[existing], func(t *testing.T) {
			s := newTestStore(t)
			u := createTestUser(t, s, 577190)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			item := xnode.Identity{UserID: u.ID, CredentialsCipher: "old-secret", SubscriptionCipher: "old-subscription"}
			var err error
			if existing {
				item, err = s.SaveXNodeIdentity(ctx, item)
				if err != nil {
					t.Fatal(err)
				}
			}
			item.CredentialsCipher = "stale-restored-secret"
			tx, err := s.db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err = tx.ExecContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, u.ID); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { _, e := s.SaveXNodeIdentity(ctx, item); done <- e }()
			blocked := false
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				if err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database()
 AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND strpos(query,'FOR SHARE')>0 AND strpos(query,'account_status')>0)`).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				if blocked {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			if !blocked {
				t.Fatal("identity write did not acquire lifecycle share lock")
			}
			if _, err = tx.ExecContext(ctx, `UPDATE users SET account_status='purged',account_generation=1 WHERE id=$1`, u.ID); err != nil {
				t.Fatal(err)
			}
			if _, err = tx.ExecContext(ctx, `UPDATE xnode_identities SET credentials_cipher='',subscription_cipher='',subscription_hash=NULL,access_disabled=1 WHERE user_id=$1`, u.ID); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(); err != nil {
				t.Fatal(err)
			}
			if err = <-done; !errors.Is(err, ErrAccountUnavailable) {
				t.Fatal("stale identity crossed purge commit", err)
			}
			got, e := s.XNodeIdentityByUser(ctx, u.ID)
			if existing {
				if e != nil || got.CredentialsCipher != "" || got.SubscriptionCipher != "" || !got.AccessDisabled {
					t.Fatal("purged secret restored", got, e)
				}
			} else if !errors.Is(e, xnode.ErrNotFound) {
				t.Fatal("new secret inserted after purge", got, e)
			}
			if _, err = s.SaveXNodeIdentity(ctx, xnode.Identity{ServiceName: "admin-service", CredentialsCipher: "independent-service"}); err != nil {
				t.Fatal("member fence blocked unrelated service identity", err)
			}
		})
	}
}
