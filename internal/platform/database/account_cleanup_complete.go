package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// CompleteAccountCleanupProbe commits only an existing exact pending nonce.
// Authentication can clear it while Bot API work waits without being undone.
func (s *Store) CompleteAccountCleanupProbe(ctx context.Context, p model.AccountCleanupProbe, o model.AccountCleanupObservation, at time.Time) (model.AccountCleanupCompletion, error) {
	zero := model.AccountCleanupCompletion{}
	if _, _, valid := classifyCleanupObservation(o, false); !valid || at.IsZero() {
		return zero, ErrConflict
	}
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (model.AccountCleanupCompletion, error) {
		h, e := lockCleanupHealth(ctx, tx, true)
		if e != nil {
			return zero, e
		}
		var current, telegram int64
		var role, status string
		e = tx.QueryRowContext(ctx, `SELECT account_generation,telegram_id,role,account_status FROM users WHERE id=$1 FOR UPDATE`, p.UserID).Scan(&current, &telegram, &role, &status)
		if errors.Is(e, sql.ErrNoRows) {
			return zero, nil
		}
		if e != nil {
			return zero, e
		}
		if s.cleanupProtected(telegram, role) {
			zero.Reason = "administrator_protected"
			return zero, nil
		}
		c, e := readCleanupCheck(ctx, tx, p.UserID)
		if errors.Is(e, sql.ErrNoRows) {
			return zero, nil
		}
		if e != nil {
			return zero, e
		}
		if status == "purged" || p.TelegramID != telegram || p.BotID != h.BotID || p.Epoch != h.Epoch || !cleanupProbeMatches(c, h, current, p.AccountGeneration, p.Nonce, at) || p.Final != c.final || !p.StartedAt.Equal(c.started.Time) {
			zero.Reason = "probe_superseded"
			return zero, nil
		}
		reachable, qualifying, _ := classifyCleanupObservation(o, c.reachable.Valid)
		if !reachable && !cleanupVerified(h, at) {
			zero.Reason = "bot_unverified"
			return zero, nil
		}
		if !c.health.Valid || at.Sub(c.health.Time) >= 10*time.Minute {
			if e = recordCleanupHealthTx(ctx, tx, !reachable, at); e != nil {
				return zero, e
			}
			if _, e = tx.ExecContext(ctx, `UPDATE account_cleanup_checks SET health_observed_at=$2 WHERE user_id=$1`, p.UserID, at.UTC()); e != nil {
				return zero, e
			}
		}
		result := model.AccountCleanupCompletion{Accepted: true, Generation: current}
		if reachable {
			if c.count > 0 || c.admission.Valid {
				if e = insertCleanupEvidence(ctx, tx, p, o, false, nil, "probe", at); e != nil {
					return zero, e
				}
			}
			_, e = tx.ExecContext(ctx, `UPDATE account_cleanup_checks SET private_reachable_at=$2,first_failure_at=NULL,last_failure_at=NULL,failure_count=0,pending_nonce=NULL,pending_started_at=NULL,pending_final=0,admission_evidence_id=NULL,next_probe_at=$3 WHERE user_id=$1`, p.UserID, at.UTC(), at.Add(24*time.Hour))
			if e == nil {
				_, e = tx.ExecContext(ctx, `UPDATE users SET bot_blocked=0 WHERE id=$1`, p.UserID)
			}
			result.Reason = "reachable"
			return result, e
		}
		if qualifying && c.final && cleanupMature(c, p.StartedAt) {
			h, e = lockCleanupHealth(ctx, tx, true)
			if e != nil {
				return zero, e
			}
			if cleanupHealthy(h, at) {
				return s.admitCleanupTx(ctx, tx, p, o, c, current, status, at)
			}
			result.Reason = "correlated_failure_pause"
		}
		if qualifying && !c.final && (!c.last.Valid || at.Sub(c.last.Time) >= 24*time.Hour) {
			if e = insertCleanupEvidence(ctx, tx, p, o, true, nil, "probe", at); e != nil {
				return zero, e
			}
			_, e = tx.ExecContext(ctx, `UPDATE account_cleanup_checks SET first_failure_at=COALESCE(first_failure_at,$2),last_failure_at=$2,failure_count=LEAST(3,failure_count+1) WHERE user_id=$1`, p.UserID, at.UTC())
			if e != nil {
				return zero, e
			}
			c, e = readCleanupCheck(ctx, tx, p.UserID)
			if e != nil {
				return zero, e
			}
		}
		next := at.Add(24 * time.Hour)
		if qualifying && !c.final && cleanupMature(c, at) {
			next = at
		}
		_, e = tx.ExecContext(ctx, `UPDATE account_cleanup_checks SET pending_nonce=NULL,pending_started_at=NULL,pending_final=0,next_probe_at=$2 WHERE user_id=$1`, p.UserID, next)
		if result.Reason == "" {
			result.Reason = "observation_recorded"
		}
		return result, e
	})
}

func insertCleanupEvidence(ctx context.Context, tx *sql.Tx, p model.AccountCleanupProbe, o model.AccountCleanupObservation, qualifying bool, purge any, source string, at time.Time) error {
	_, e := tx.ExecContext(ctx, `INSERT INTO account_cleanup_evidence(nonce,user_id,account_generation,bot_id,bot_epoch,started_at,completed_at,source,action_class,chat_class,qualifying,purge_generation) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, p.Nonce, p.UserID, p.AccountGeneration, p.BotID, p.Epoch, p.StartedAt.UTC(), at.UTC(), source, o.SendAction, o.GetChat, boolInt(qualifying), purge)
	return e
}
