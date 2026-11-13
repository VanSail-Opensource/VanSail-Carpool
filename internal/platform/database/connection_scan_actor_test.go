package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func scanAdministrator(t *testing.T, store *Store) model.User {
	t.Helper()
	actor, _, err := store.UpsertTelegramUser(context.Background(), model.TelegramProfile{ID: 900699, FirstName: "Scan administrator"}, true)
	if err != nil {
		t.Fatal(err)
	}
	return actor
}

func TestConnectionScanAdmissionRechecksActorAndPreservesAcceptedIntent(t *testing.T) {
	for _, state := range []string{"demoted", "blocked", "purged", "revived"} {
		t.Run(state, func(t *testing.T) {
			ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
			owner, actor := createTestUser(t, store, 900698), scanAdministrator(t, store)
			input := providerops.ConnectionScanInput{ActorUserID: actor.ID, ActorAccountGeneration: actor.AccountGeneration, UserID: owner.ID, IdempotencyKey: "accepted", RequestFingerprint: "scan-admin-proof", ExpiresAt: now.Add(time.Minute)}
			accepted, _, err := store.CreateConnectionScan(ctx, input, now)
			if err != nil {
				t.Fatal(err)
			}
			statement := `UPDATE users SET role='user' WHERE id=$1`
			if state == "blocked" {
				statement = `UPDATE users SET bot_blocked=1 WHERE id=$1`
			} else if state == "purged" {
				statement = `UPDATE users SET account_status='purged' WHERE id=$1`
			} else if state == "revived" {
				statement = `UPDATE users SET account_generation=account_generation+1 WHERE id=$1`
			}
			if _, err := store.DB().ExecContext(ctx, statement, actor.ID); err != nil {
				t.Fatal(err)
			}
			input.IdempotencyKey = "new-after-authority-change"
			if _, _, err := store.CreateConnectionScan(ctx, input, now); !errors.Is(err, connections.ErrScanForbidden) && !errors.Is(err, ErrAccountUnavailable) {
				t.Fatal("stale administrator admitted fresh scan", err)
			}
			var scans, jobs, audits int
			if err := store.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM connection_scans),(SELECT COUNT(*) FROM outbox_jobs WHERE kind='connection_scan_request'),(SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1 AND action='connection_scan.create' AND target_id=$2)`, actor.ID, accepted.ID).Scan(&scans, &jobs, &audits); err != nil || scans != 1 || jobs != 1 || audits != 1 {
				t.Fatal("admission or audit was not atomic", scans, jobs, audits, err)
			}
			// Accepted read-only intent remains executable; every later API poll
			// independently authorizes its current requesting administrator.
			started, err := store.StartConnectionScan(ctx, accepted.ID, now)
			if err != nil || started.Status != providerops.StatusProcessing {
				t.Fatal("accepted diagnostic intent was canceled", started, err)
			}
		})
	}
}

func TestConnectionScanAuditFailureRollsBackAdmission(t *testing.T) {
	ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
	owner, actor := createTestUser(t, store, 900697), scanAdministrator(t, store)
	if _, err := store.DB().ExecContext(ctx, `CREATE FUNCTION reject_scan_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='connection_scan.create' THEN RAISE EXCEPTION 'injected audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_scan_audit BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION reject_scan_audit()`); err != nil {
		t.Fatal(err)
	}
	input := providerops.ConnectionScanInput{ActorUserID: actor.ID, ActorAccountGeneration: actor.AccountGeneration, UserID: owner.ID, IdempotencyKey: "audit-recovery", RequestFingerprint: "scan-audit-proof", ExpiresAt: now.Add(time.Minute)}
	if _, _, err := store.CreateConnectionScan(ctx, input, now); err == nil {
		t.Fatal("admission ignored failed audit")
	}
	var scans, jobs int
	if err := store.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM connection_scans),(SELECT COUNT(*) FROM outbox_jobs WHERE kind='connection_scan_request')`).Scan(&scans, &jobs); err != nil || scans != 0 || jobs != 0 {
		t.Fatal("failed audit left scan work", scans, jobs, err)
	}
	if _, err := store.DB().ExecContext(ctx, `DROP TRIGGER reject_scan_audit ON audit_events`); err != nil {
		t.Fatal(err)
	}
	if _, replayed, err := store.CreateConnectionScan(ctx, input, now); err != nil || replayed {
		t.Fatal("failed admission poisoned the retry", replayed, err)
	}
}
