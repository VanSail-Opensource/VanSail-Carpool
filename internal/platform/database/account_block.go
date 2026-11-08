package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// HandleTelegramBotStatus records ordered private-chat evidence. A first block
// never revokes benefits, increments generation, closes a wallet or admits purge.
func (s *Store) retryAttempt_HandleTelegramBotStatus(ctx context.Context, telegramID, updateID, eventAt int64, blocked, protected bool, now time.Time) error {
	if protected {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id, role string
	var previousEvent, previousUpdate int64
	err = tx.QueryRowContext(ctx, `SELECT id,role,bot_event_at,bot_update_id FROM users WHERE telegram_id=$1 FOR UPDATE`, telegramID).Scan(&id, &role, &previousEvent, &previousUpdate)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if role == "admin" || s.cleanupConfiguredAdmin(telegramID) || eventAt < previousEvent || eventAt == previousEvent && updateID <= previousUpdate {
		return nil
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET bot_blocked=$1,bot_event_at=$2,bot_update_id=$3 WHERE id=$4`, boolInt(blocked), eventAt, updateID, id); err != nil {
		return err
	}
	if !blocked {
		if err = cancelCleanupCandidateTx(ctx, tx, id, "unblocked", now); err != nil {
			return err
		}
	}
	if blocked {
		if _, err = tx.ExecContext(ctx, `UPDATE account_cleanup_checks SET next_probe_at=LEAST(next_probe_at,$2) WHERE user_id=$1 AND pending_nonce IS NULL`, id, now.UTC()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const accountJobTargets = `((CASE ((payload)::jsonb #> '{userId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (payload)::jsonb #>> '{userId}' END)=$1
 OR (CASE ((payload)::jsonb #> '{purchaseId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (payload)::jsonb #>> '{purchaseId}' END) IN (SELECT id FROM purchases WHERE user_id=$2)
 OR (CASE ((payload)::jsonb #> '{accountId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (payload)::jsonb #>> '{accountId}' END) IN (SELECT id FROM emby_accounts WHERE user_id=$3)
 OR (CASE ((payload)::jsonb #> '{operationId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (payload)::jsonb #>> '{operationId}' END) IN (SELECT id FROM provider_operations WHERE owner_user_id=$4 OR actor_user_id=$5))`

func cancelAccountQueuedJobsTx(ctx context.Context, tx *sql.Tx, id string) error {
	return cancelAccountKnownJobsTx(ctx, tx, id)
}
