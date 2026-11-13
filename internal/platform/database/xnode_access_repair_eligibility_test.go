package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestAccessRepairSkipsUnavailableCredentialProvisioning(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	now := time.Date(2026, 11, 13, 14, 0, 0, 0, time.UTC)
	// The authoritative lifecycle applies even if retained onboarding says
	// complete. Credential provisioning also requires a bot-unblocked owner.
	_, err := store.DB().ExecContext(ctx, `INSERT INTO users
		(id,telegram_id,onboarding_state,account_status,bot_blocked,created_at,updated_at)
		VALUES('purged',92001,'complete','purged',1,$1,$1),
		('purging',92002,'complete','purging',1,$1,$1),
		('active-blocked',92003,'complete','active',1,$1,$1)`, stamp(now))
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.DB().ExecContext(ctx, `INSERT INTO users
		(id,telegram_id,onboarding_state,created_at,updated_at) VALUES('z-live',92200,'complete',$1,$1)`, stamp(now))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := store.EnqueueXNodeAccessRepairs(ctx, now); err != nil {
			t.Fatal(err)
		}
	}
	var all, live, blocked int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER(WHERE payload::jsonb->>'userId'='z-live'),
		COUNT(*) FILTER(WHERE payload::jsonb->>'userId'='active-blocked')
		FROM outbox_jobs WHERE kind=$1`, xnode.AccessRepairJob).Scan(&all, &live, &blocked); err != nil || all != 1 || live != 1 || blocked != 0 {
		t.Fatalf("repair jobs = %d total, %d live, %d blocked active: %v", all, live, blocked, err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE users SET account_status='active',bot_blocked=0 WHERE id IN('purging','active-blocked')`); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueueXNodeAccessRepairs(ctx, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var restored int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=$1 AND payload::jsonb->>'userId' IN('purging','active-blocked')`, xnode.AccessRepairJob).Scan(&restored); err != nil || restored != 2 {
		t.Fatalf("restored subject repair jobs = %d: %v", restored, err)
	}
}

func TestAccessRepairPageSkipsConflictedOwnersBeforeLimit(t *testing.T) {
	for _, scenario := range []string{"pending-review", "temporary-ban", "missing-blocked-credentials"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, store := context.Background(), newTestStore(t)
			now := time.Now().UTC()
			if _, err := store.db.ExecContext(ctx, `INSERT INTO users(id,telegram_id,onboarding_state,created_at,updated_at)
 SELECT '000-conflicted-'||lpad(i::text,3,'0'),982000+i,'complete',$1,$1 FROM generate_series(1,128) i`, stamp(now)); err != nil {
				t.Fatal(err)
			}
			if _, err := store.db.ExecContext(ctx, `INSERT INTO users(id,telegram_id,onboarding_state,created_at,updated_at)
 VALUES('zz-live',983000,'complete',$1,$1)`, stamp(now)); err != nil {
				t.Fatal(err)
			}
			kind, status := "purchase_refund", "pending_review"
			if scenario == "temporary-ban" {
				kind, status = "admin_temporary_ban", "succeeded"
			}
			if _, err := store.db.ExecContext(ctx, `INSERT INTO provider_operations
 (id,actor_user_id,owner_user_id,kind,idempotency_key,request_fingerprint,status,created_at,updated_at)
 SELECT 'operation-'||id,id,id,$1,'retained-conflict','retained-conflict-fingerprint',$2,$3,$3
 FROM users WHERE id LIKE '000-conflicted-%'`, kind, status, stamp(now)); err != nil {
				t.Fatal(err)
			}
			if scenario == "missing-blocked-credentials" {
				if _, err := store.db.ExecContext(ctx, `UPDATE provider_operations SET status='succeeded'`); err != nil {
					t.Fatal(err)
				}
				if _, err := store.db.ExecContext(ctx, `UPDATE users SET bot_blocked=1 WHERE id LIKE '000-conflicted-%'`); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "temporary-ban" {
				if _, err := store.db.ExecContext(ctx, `INSERT INTO admin_temporary_bans
 (user_id,actor_user_id,reason,expires_at,ban_operation_id,created_at,updated_at)
 SELECT id,id,'retained ban',$1,'operation-'||id,$2,$2 FROM users WHERE id LIKE '000-conflicted-%'`, stamp(now.Add(time.Hour)), stamp(now)); err != nil {
					t.Fatal(err)
				}
			}
			for range 2 {
				if err := store.EnqueueXNodeAccessRepairs(ctx, now); err != nil {
					t.Fatal(err)
				}
			}
			var all, live int
			if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER(WHERE payload::jsonb->>'userId'='zz-live')
 FROM outbox_jobs WHERE kind=$1`, xnode.AccessRepairJob).Scan(&all, &live); err != nil || all != 1 || live != 1 {
				t.Fatal("conflicted owners starved or escaped bounded repair", all, live, err)
			}
		})
	}
}
