package database

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type authenticatedCleanupResult struct {
	user                 model.User
	existed, unavailable bool
}

func (s *Store) upsertAuthenticatedTelegram(ctx context.Context, p model.TelegramProfile, configured bool) (model.User, bool, error) {
	if p.ID <= 0 || p.ID >= 1<<52 {
		return model.User{}, false, ErrConflict
	}
	now := time.Now().UTC()
	result, err := hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (authenticatedCleanupResult, error) {
		r := authenticatedCleanupResult{}
		// Also serialize first-time inserts, before a users row exists.
		if _, e := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "account-auth:"+strconv.FormatInt(p.ID, 10)); e != nil {
			return r, e
		}
		var id, role, status string
		var generation int64
		e := tx.QueryRowContext(ctx, `SELECT id,role,account_status,account_generation FROM users WHERE telegram_id=$1 FOR UPDATE`, p.ID).Scan(&id, &role, &status, &generation)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return r, e
		}
		r.existed = e == nil
		admin := configured || s.cleanupConfiguredAdmin(p.ID) || role == "admin"
		if r.existed {
			if e = cancelCleanupCandidateTx(ctx, tx, id, "authenticated", now); e != nil {
				return r, e
			}
			if status == "purging" && !admin {
				_, e = tx.ExecContext(ctx, `UPDATE users SET bot_blocked=0,recovery_reason='account_cleanup_pending' WHERE id=$1`, id)
				r.unavailable = true
				return r, e // Commit cancellation despite refusing partial-state login.
			}
			if status == "purging" && admin {
				if generation == math.MaxInt64 {
					return r, ErrConflict
				}
				generation++
			}
			if status != "active" {
				_, e = tx.ExecContext(ctx, `UPDATE users SET account_status='active',account_generation=$2,bot_blocked=0,onboarding_state='intro',policy_accepted_at=NULL,accepted_agreement_revision=0,created_at=$3,updated_at=$3 WHERE id=$1`, id, generation, stamp(now))
				if e != nil {
					return r, e
				}
			}
		}
		role = "user"
		if admin {
			role = "admin"
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO users(id,telegram_id,telegram_first_name,telegram_last_name,telegram_username,role,new_user,notification_locale,auto_traffic_reset_enabled,created_at,updated_at)
 VALUES($1,$2,$3,$4,$5,$6,0,$7,0,$8,$8) ON CONFLICT(telegram_id) DO UPDATE SET telegram_first_name=excluded.telegram_first_name,telegram_last_name=excluded.telegram_last_name,telegram_username=excluded.telegram_username,role=CASE WHEN users.role='admin' THEN 'admin' ELSE excluded.role END,new_user=0,notification_locale=excluded.notification_locale,bot_blocked=0,updated_at=excluded.updated_at
 WHERE users.account_status='active'`, strconv.FormatInt(p.ID, 10), p.ID, p.FirstName, p.LastName, p.Username, role, affiliates.NormalizeLocale(p.LanguageCode), stamp(now))
		if e != nil {
			return r, e
		}
		r.user, e = scanUser(tx.QueryRowContext(ctx, userSelect+` WHERE users.telegram_id=$1`, p.ID))
		if e != nil {
			return r, e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO balances(user_id,txb_minor,updated_at) VALUES($1,0,$2) ON CONFLICT(user_id) DO NOTHING`, r.user.ID, stamp(now))
		return r, e
	})
	if err != nil {
		return model.User{}, false, err
	}
	if result.unavailable {
		return model.User{}, false, ErrAccountCleanupRecoveryPending
	}
	return result.user, !result.existed, nil
}

func (s *Store) authenticatedSession(ctx context.Context, hash []byte, scope string, at time.Time) (model.User, time.Time, error) {
	type result struct {
		user    model.User
		expires time.Time
	}
	r, err := hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (result, error) {
		var r result
		var raw string
		var e error
		r.user, e = scanUser(tx.QueryRowContext(ctx, userSelect+` JOIN sessions ON sessions.user_id=users.id WHERE sessions.token_hash=$1 AND sessions.scope=$2 AND users.account_status='active' FOR UPDATE OF users`, hash, scope))
		if e != nil {
			return r, e
		}
		e = tx.QueryRowContext(ctx, `SELECT expires_at FROM sessions WHERE token_hash=$1 AND scope=$2`, hash, scope).Scan(&raw)
		if e != nil {
			return r, e
		}
		if r.expires, e = parseStamp(raw); e != nil {
			return r, e
		}
		// Authentication must compare exact instants before recording activity.
		// RFC3339Nano text order differs at whole and fractional seconds.
		if !r.expires.After(at) {
			return r, ErrNotFound
		}
		if e = cancelCleanupCandidateTx(ctx, tx, r.user.ID, "authenticated", at); e != nil {
			return r, e
		}
		if r.user.BotBlocked {
			if _, e = tx.ExecContext(ctx, `UPDATE users SET bot_blocked=0 WHERE id=$1`, r.user.ID); e != nil {
				return r, e
			}
			r.user.BotBlocked = false
		}
		return r, nil
	})
	return r.user, r.expires, err
}
