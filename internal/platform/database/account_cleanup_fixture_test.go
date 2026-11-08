package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// Simulate a committed schedule at an explicitly controlled historical instant.
// Replies still pass the production nonce/gen/epoch/protection/spacing gates.
func cleanupTestProbe(t *testing.T, s *Store, u model.User, at time.Time, final bool) model.AccountCleanupProbe {
	t.Helper()
	ctx := context.Background()
	at = at.UTC().Truncate(time.Microsecond)
	if err := s.ConfigureAccountCleanupAdmins(nil); err != nil {
		t.Fatal(err)
	}
	h, err := s.SetAccountCleanupBot(ctx, 900001, at)
	if err != nil {
		t.Fatal(err)
	}
	nonce, err := ids.New()
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB().ExecContext(ctx, `INSERT INTO account_cleanup_checks(user_id,account_generation,bot_epoch,next_probe_at,pending_nonce,pending_started_at,pending_final) VALUES($1,$2,$3,$4,$5,$4,$6)
 ON CONFLICT(user_id) DO UPDATE SET pending_nonce=$5,pending_started_at=$4,pending_final=$6`, u.ID, u.AccountGeneration, h.Epoch, at, nonce, boolInt(final))
	if err != nil {
		t.Fatal(err)
	}
	p := model.AccountCleanupProbe{UserID: u.ID, TelegramID: u.TelegramID, BotID: h.BotID, Epoch: h.Epoch, Nonce: nonce, Final: final}
	if err = s.DB().QueryRowContext(ctx, `SELECT account_generation,pending_started_at,private_reachable_at IS NOT NULL FROM account_cleanup_checks WHERE user_id=$1`, u.ID).Scan(&p.AccountGeneration, &p.StartedAt, &p.PriorPrivateReachable); err != nil {
		t.Fatal(err)
	}
	return p
}

func cleanupTestComplete(t *testing.T, s *Store, p model.AccountCleanupProbe, o model.AccountCleanupObservation, at time.Time) model.AccountCleanupCompletion {
	t.Helper()
	result, err := s.CompleteAccountCleanupProbe(context.Background(), p, o, at.UTC().Truncate(time.Microsecond))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func admitTestAccountCleanup(t *testing.T, s *Store, u model.User, at time.Time) {
	t.Helper()
	at = at.UTC().Truncate(time.Microsecond)
	start := at.Add(-7*24*time.Hour - time.Minute)
	if _, err := s.DB().ExecContext(context.Background(), `UPDATE users SET created_at=$2 WHERE id=$1`, u.ID, stamp(start.Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}
	negative := model.AccountCleanupObservation{SendAction: "blocked", GetChat: "private"}
	for _, observed := range []time.Time{start, start.Add(24 * time.Hour), at.Add(-time.Minute)} {
		result := cleanupTestComplete(t, s, cleanupTestProbe(t, s, u, observed, false), negative, observed)
		if !result.Accepted || result.Admitted {
			t.Fatal("invalid historical failure", result)
		}
	}
	result := cleanupTestComplete(t, s, cleanupTestProbe(t, s, u, at, true), negative, at)
	if !result.Admitted {
		t.Fatal("qualified separate final not admitted", result)
	}
}
