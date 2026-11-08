package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"hash/fnv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// EnqueueAccountCleanupChecks staggers ordinary accounts over the UTC day and
// commits each nonce together with its existing-outbox job before any API call.
func (s *Store) EnqueueAccountCleanupChecks(ctx context.Context, at time.Time, limit int) error {
	if limit < 1 || limit > 64 || at.IsZero() {
		return ErrConflict
	}
	_, err := hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (bool, error) {
		h, e := lockCleanupHealth(ctx, tx, true)
		if e != nil || !cleanupVerified(h, at) || s.cleanupAdmins.Load() == nil {
			return false, e
		}
		admins := make([]int64, 0, len(s.cleanupAdmins.Load().ids))
		for id := range s.cleanupAdmins.Load().ids {
			admins = append(admins, id)
		}
		rows, e := tx.QueryContext(ctx, `SELECT u.id,u.telegram_id,u.account_generation,u.role FROM users u
 LEFT JOIN account_cleanup_checks c ON c.user_id=u.id
 WHERE u.role='user' AND u.account_status IN('active','purging') AND u.telegram_id>0 AND u.telegram_id<4503599627370496
 AND NOT (u.telegram_id=ANY($4::bigint[]))
 AND (c.user_id IS NULL OR c.bot_epoch<>$1 OR c.next_probe_at<=$2)
 ORDER BY COALESCE(c.next_probe_at,'-infinity'::timestamptz),u.id LIMIT $3 FOR UPDATE OF u SKIP LOCKED`, h.Epoch, at.UTC(), limit, admins)
		if e != nil {
			return false, e
		}
		type owner struct {
			id                   string
			telegram, generation int64
			role                 string
		}
		owners := []owner{}
		for rows.Next() {
			var o owner
			if e = rows.Scan(&o.id, &o.telegram, &o.generation, &o.role); e != nil {
				rows.Close()
				return false, e
			}
			owners = append(owners, o)
		}
		if e = finishCompileRows(rows); e != nil {
			return false, e
		}
		for _, o := range owners {
			if s.cleanupProtected(o.telegram, o.role) {
				continue
			}
			c, e := readCleanupCheck(ctx, tx, o.id)
			if errors.Is(e, sql.ErrNoRows) || e == nil && (c.epoch != h.Epoch || c.generation != o.generation && !c.admission.Valid) {
				next := cleanupDailySlot(o.id, at)
				_, e = tx.ExecContext(ctx, `INSERT INTO account_cleanup_checks(user_id,account_generation,bot_epoch,next_probe_at) VALUES($1,$2,$3,$4)
 ON CONFLICT(user_id) DO UPDATE SET account_generation=$2,bot_epoch=$3,next_probe_at=$4,private_reachable_at=NULL,health_observed_at=NULL,first_failure_at=NULL,last_failure_at=NULL,failure_count=0,pending_nonce=NULL,pending_started_at=NULL,pending_final=0,admission_evidence_id=NULL`, o.id, o.generation, h.Epoch, next)
				if e != nil {
					return false, e
				}
				c, e = readCleanupCheck(ctx, tx, o.id)
			}
			if e != nil {
				return false, e
			}
			if at.Before(c.next) || c.nonce.Valid && c.started.Valid && at.Sub(c.started.Time) < cleanupProbeTTL {
				continue
			}
			jobID, processing, e := cleanupProbeJobTx(ctx, tx, o.id, c.generation)
			if e != nil {
				return false, e
			}
			if processing {
				if _, e = tx.ExecContext(ctx, `UPDATE account_cleanup_checks SET next_probe_at=$2 WHERE user_id=$1`, o.id, at.Add(time.Minute)); e != nil {
					return false, e
				}
				continue
			}
			nonce, e := ids.New()
			if e != nil {
				return false, e
			}
			final := cleanupMature(c, at)
			if _, e = tx.ExecContext(ctx, `UPDATE account_cleanup_checks SET pending_nonce=$2,pending_started_at=$3,pending_final=$4,next_probe_at=$5 WHERE user_id=$1`, o.id, nonce, at.UTC(), boolInt(final), at.Add(cleanupProbeTTL)); e != nil {
				return false, e
			}
			body, e := json.Marshal(map[string]any{"userId": o.id, "generation": c.generation, "nonce": nonce})
			if e != nil {
				return false, e
			}
			if e = writeCleanupProbeJobTx(ctx, tx, jobID, string(body), at); e != nil {
				return false, e
			}
		}
		return true, nil
	})
	return err
}

func cleanupDailySlot(id string, at time.Time) time.Time {
	h := fnv.New64a()
	_, _ = h.Write([]byte(id))
	utc := at.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC).Add(time.Duration(h.Sum64()%86400) * time.Second)
}

// AccountCleanupProbe refuses canceled, expired, foreign-bot or protected work.
func (s *Store) AccountCleanupProbe(ctx context.Context, user string, generation int64, nonce string) (*model.AccountCleanupProbe, error) {
	return hourlyCreditTransaction(ctx, s, func(tx *sql.Tx) (*model.AccountCleanupProbe, error) {
		h, e := lockCleanupHealth(ctx, tx, false)
		if e != nil || !cleanupVerified(h, time.Now().UTC()) {
			return nil, e
		}
		var telegram, current int64
		var role, status string
		e = tx.QueryRowContext(ctx, `SELECT telegram_id,account_generation,role,account_status FROM users WHERE id=$1 FOR UPDATE`, user).Scan(&telegram, &current, &role, &status)
		if errors.Is(e, sql.ErrNoRows) {
			return nil, nil
		}
		if e != nil {
			return nil, e
		}
		if s.cleanupProtected(telegram, role) || status == "purged" {
			return nil, nil
		}
		c, e := readCleanupCheck(ctx, tx, user)
		if errors.Is(e, sql.ErrNoRows) {
			return nil, nil
		}
		if e != nil {
			return nil, e
		}
		now := time.Now().UTC()
		if !cleanupProbeScopeMatches(c, h, current, generation, nonce) || now.Before(c.started.Time) {
			return nil, nil
		}
		if now.Sub(c.started.Time) > cleanupProbeTTL {
			// Backlog is not a failed account. Re-admit only this still-existing
			// nonce before API calls; old captured StartedAt can never complete it.
			if e = tx.QueryRowContext(ctx, `UPDATE account_cleanup_checks SET pending_started_at=$2,next_probe_at=$3 WHERE user_id=$1 AND pending_nonce=$4 RETURNING pending_started_at`, user, now, now.Add(cleanupProbeTTL), nonce).Scan(&c.started.Time); e != nil {
				return nil, e
			}
		}
		return &model.AccountCleanupProbe{UserID: user, TelegramID: telegram, AccountGeneration: generation, BotID: h.BotID, Epoch: h.Epoch, Nonce: nonce, StartedAt: c.started.Time, Final: c.final, PriorPrivateReachable: c.reachable.Valid}, nil
	})
}

func cleanupProbeMatches(c cleanupCheck, h model.AccountCleanupHealth, current, generation int64, nonce string, at time.Time) bool {
	return cleanupProbeScopeMatches(c, h, current, generation, nonce) && !at.Before(c.started.Time) && at.Sub(c.started.Time) <= cleanupProbeTTL
}

func cleanupProbeScopeMatches(c cleanupCheck, h model.AccountCleanupHealth, current, generation int64, nonce string) bool {
	return c.epoch == h.Epoch && c.generation == generation && c.nonce.Valid && c.nonce.String == nonce && c.started.Valid && (current == generation || c.admission.Valid && current == generation+1)
}
