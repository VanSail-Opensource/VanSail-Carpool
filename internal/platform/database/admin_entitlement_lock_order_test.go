package database

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestManualGrantContentionTakesOwnerLockWithoutUpgrade(t *testing.T) {
	for _, mode := range []string{"same owner", "self grant", "reciprocal administrators"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			store, actor, owner, combo, now := manualGrantFixture(t)
			first := manualGrantInput(actor.ID, owner.ID, combo.ID, now)
			if mode == "self grant" {
				first.UserID = actor.ID
			}
			second := first
			if mode == "reciprocal administrators" {
				if _, err := store.DB().Exec(`UPDATE users SET role='admin' WHERE id=$1`, owner.ID); err != nil {
					t.Fatal(err)
				}
				second.ActorUserID, second.UserID = owner.ID, actor.ID
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			gate, err := store.DB().BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
			if err != nil {
				t.Fatal(err)
			}
			defer gate.Rollback()
			if _, err := gate.ExecContext(ctx, `SELECT id FROM users WHERE id=$1 OR id=$2 ORDER BY id FOR SHARE`, actor.ID, owner.ID); err != nil {
				t.Fatal(err)
			}
			type outcome struct {
				input    AdminEntitlementCreateInput
				purchase model.Purchase
				err      error
			}
			done := make(chan outcome, 2)
			for _, input := range []AdminEntitlementCreateInput{first, second} {
				go func() {
					purchase, err := store.createAdminEntitlementAttempt(ctx, input, now)
					done <- outcome{input, purchase, err}
				}()
			}
			received := 0
			defer func() {
				cancel()
				_ = gate.Rollback()
				for received < 2 {
					<-done
					received++
				}
			}()
			// Both attempts must reach account contention before releasing the
			// gate. Old SHARE-then-upgrade admission forms a real lock cycle here.
			waitManualGrantAccountWaiters(t, ctx, store, 2)
			if err := gate.Rollback(); err != nil {
				t.Fatal(err)
			}
			accepted := map[string]model.Purchase{}
			for received < 2 {
				result := <-done
				received++
				if result.err != nil {
					var pg *pgconn.PgError
					// A stale serializable snapshot can require the normal whole
					// attempt retry. A deadlock is never an acceptable lock order.
					if !errors.As(result.err, &pg) || pg.Code != "40001" {
						t.Fatal("manual grant account locks deadlocked or rejected admission", result.err)
					}
					continue
				}
				if prior, ok := accepted[result.input.UserID]; ok && !reflect.DeepEqual(prior, result.purchase) {
					t.Fatal("contended replay changed original result", prior, result.purchase)
				}
				accepted[result.input.UserID] = result.purchase
			}
			for _, input := range []AdminEntitlementCreateInput{first, second} {
				purchase, err := store.CreateAdminEntitlement(ctx, input, now)
				if err != nil {
					t.Fatal("accepted grant did not recover through its public retry path", err)
				}
				if prior, ok := accepted[input.UserID]; ok && !reflect.DeepEqual(prior, purchase) {
					t.Fatal("recovered grant changed original result", prior, purchase)
				}
				accepted[input.UserID] = purchase
			}
			var purchases, audits, jobs int
			if err := store.DB().QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM purchases),
				(SELECT COUNT(*) FROM audit_events WHERE action='entitlement.create'),
				(SELECT COUNT(*) FROM outbox_jobs WHERE kind='xnode_sync_user')`).Scan(&purchases, &audits, &jobs); err != nil || purchases != len(accepted) || audits != len(accepted) || jobs != len(accepted) {
				t.Fatal("contended grant duplicated durable effects", purchases, audits, jobs, err)
			}
		})
	}
}

func waitManualGrantAccountWaiters(t *testing.T, ctx context.Context, store *Store, count int) {
	t.Helper()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting int
		if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pg_stat_activity
			WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock'
			AND strpos(query,'FROM users WHERE')>0`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting >= count {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("manual grants did not reach the controlled account contention", ctx.Err())
		case <-ticker.C:
		}
	}
}

func TestManualGrantBlockedOwnerDoesNotSerializeUnrelatedOwner(t *testing.T) {
	t.Parallel()
	store, actor, owner, combo, now := manualGrantFixture(t)
	// Force the actor to precede the blocked owner, so an unnecessarily
	// exclusive actor lock would also block the unrelated owner's grant.
	if actor.ID > owner.ID {
		actor, owner = owner, actor
		if _, err := store.DB().Exec(`UPDATE users SET role='admin' WHERE id=$1`, actor.ID); err != nil {
			t.Fatal(err)
		}
	}
	other := createTestUser(t, store, 42_003)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	gate, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Rollback()
	if _, err := gate.ExecContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, owner.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := store.CreateAdminEntitlement(ctx, manualGrantInput(actor.ID, owner.ID, combo.ID, now), now)
		done <- err
	}()
	finished := false
	defer func() {
		cancel()
		_ = gate.Rollback()
		if !finished {
			<-done
		}
	}()
	waitManualGrantAccountWaiters(t, ctx, store, 1)
	independent, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	if _, err := store.CreateAdminEntitlement(independent, manualGrantInput(actor.ID, other.ID, combo.ID, now), now); err != nil {
		t.Fatal("one blocked owner serialized an unrelated grant by the same actor", err)
	}
	if err := gate.Rollback(); err != nil {
		t.Fatal(err)
	}
	err = <-done
	finished = true
	if err != nil {
		t.Fatal("blocked grant did not resume", err)
	}
}
