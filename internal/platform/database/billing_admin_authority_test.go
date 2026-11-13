package database

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestBillingAdministratorAuthorityFencesFreshCreditAndLimits(t *testing.T) {
	for _, state := range []string{"demoted", "purging", "blocked"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			ctx, s := context.Background(), newTestStore(t)
			actor := createTestAdmin(t, s, 970011)
			owner := createTestUser(t, s, 970012)
			now := time.Now().UTC()
			first := createTestPaymentOrder(t, s, owner.ID, "ezpay", 125, now)
			if err := s.ExpirePaymentOrder(ctx, first.ID, "ezpay", now); err != nil {
				t.Fatal(err)
			}
			credit, err := s.CourtesyCreditPayment(ctx, actor.ID, first.ID, "confirmed provider failure", now)
			if err != nil {
				t.Fatal(err)
			}
			fresh := createTestPaymentOrder(t, s, owner.ID, "ezpay", 125, now)
			if err = s.ExpirePaymentOrder(ctx, fresh.ID, "ezpay", now); err != nil {
				t.Fatal(err)
			}
			beforeBounds, err := s.AddTXBBounds(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var before, after string
			const effects = `SELECT jsonb_build_array(
				(SELECT txb_minor FROM balances WHERE user_id=$1),
				(SELECT count(*) FROM ledger_entries WHERE user_id=$1),
				(SELECT count(*) FROM courtesy_credits),
				(SELECT count(*) FROM audit_events WHERE actor_user_id=$2),
				(SELECT count(*) FROM user_notification_events WHERE user_id=$1),
				(SELECT count(*) FROM outbox_jobs))::text`
			if err = s.DB().QueryRowContext(ctx, effects, owner.ID, actor.ID).Scan(&before); err != nil {
				t.Fatal(err)
			}
			switch state {
			case "demoted":
				_, err = s.DB().ExecContext(ctx, `UPDATE users SET role='user' WHERE id=$1`, actor.ID)
			case "purging":
				_, err = s.DB().ExecContext(ctx, `UPDATE users SET account_status='purging' WHERE id=$1`, actor.ID)
			case "blocked":
				_, err = s.DB().ExecContext(ctx, `UPDATE users SET bot_blocked=1 WHERE id=$1`, actor.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.CourtesyCreditPayment(ctx, actor.ID, fresh.ID, "new provider failure", now); !errors.Is(err, ErrConflict) {
				t.Fatal("fresh credit accepted stale administrator", err)
			}
			if _, err = s.UpdateAddTXBBounds(ctx, 250, 25000, actor.ID, now); !errors.Is(err, ErrConflict) {
				t.Fatal("billing bounds accepted stale administrator", err)
			}
			replayed, err := s.CourtesyCreditPayment(ctx, actor.ID, first.ID, "read committed receipt", now.Add(time.Minute))
			if err != nil || !replayed.Replayed || replayed.ID != credit.ID || replayed.LedgerEntryID != credit.LedgerEntryID || replayed.ActorUserID != actor.ID {
				t.Fatal("retained courtesy receipt lost after authority change", replayed, err)
			}
			afterBounds, err := s.AddTXBBounds(ctx)
			if err != nil || !reflect.DeepEqual(beforeBounds, afterBounds) {
				t.Fatal("refused write changed limits", afterBounds, err)
			}
			if err = s.DB().QueryRowContext(ctx, effects, owner.ID, actor.ID).Scan(&after); err != nil || before != after {
				t.Fatal("refusal or replay changed financial/evidence effects", before, after, err)
			}
			if _, err = s.DB().ExecContext(ctx, `UPDATE users SET role='admin',account_status='active',bot_blocked=0 WHERE id=$1`, actor.ID); err != nil {
				t.Fatal(err)
			}
			if _, err = s.CourtesyCreditPayment(ctx, actor.ID, fresh.ID, "new provider failure", now); err != nil {
				t.Fatal("restored administrator could not credit unchanged target", err)
			}
			if _, err = s.UpdateAddTXBBounds(ctx, 250, 25000, actor.ID, now); err != nil {
				t.Fatal("restored administrator could not save limits", err)
			}
		})
	}
}
