package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// HandleTelegramBotStatus consumes an authenticated private-chat status event.
// Event time precedes update ID because Telegram can reseed IDs after inactivity.
func (s *Store) retryAttempt_HandleTelegramBotStatus(ctx context.Context, telegramID, updateID, eventAt int64, blocked, protected bool, now time.Time) error {
	if protected {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id, role, state string
	var generation, previousEvent, previousUpdate int64
	err = tx.QueryRowContext(ctx, `SELECT id,role,account_status,account_generation,bot_event_at,bot_update_id FROM users WHERE telegram_id=$1 FOR UPDATE`, telegramID).Scan(&id, &role, &state, &generation, &previousEvent, &previousUpdate)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if role == "admin" || eventAt < previousEvent || eventAt == previousEvent && updateID <= previousUpdate {
		return nil
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET bot_blocked=$1,bot_event_at=$2,bot_update_id=$3 WHERE id=$4`, boolInt(blocked), eventAt, updateID, id); err != nil {
		return err
	}
	if !blocked || state != "active" {
		return tx.Commit()
	}
	generation++
	if _, err = tx.ExecContext(ctx, `UPDATE users SET account_status='purging',account_generation=$1,last_purged_at=$2,onboarding_state='intro',updated_at=$3 WHERE id=$4`, generation, stamp(now), stamp(now), id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=$1`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM browser_access_links WHERE user_id=$1`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_identities SET access_disabled=1,subscription_hash=NULL,subscription_cipher='' WHERE user_id=$1 OR proxy_user_id IN (SELECT proxy_user_id FROM xnode_payg_accounts WHERE user_id=$2)`, id, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE purchases SET status='cancelled',auto_renew_enabled=0,updated_at=$1 WHERE user_id=$2 AND status IN ('active','activating','queued')`, stamp(now), id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE payment_orders SET cancelled_at=COALESCE(cancelled_at,$1),cancel_reason='ACCOUNT_PURGED',payment_url=NULL,qr_payload=NULL WHERE user_id=$2 AND status IN ('creating','pending')`, stamp(now), id); err != nil {
		return err
	}
	if err = cancelAccountQueuedJobsTx(ctx, tx, id); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{"userId": id, "generation": generation})
	if err != nil {
		return err
	}
	if err = enqueueAccountPurgeTx(ctx, tx, id, generation, now); err != nil {
		return err
	}
	if err = insertAuditTx(ctx, tx, fmt.Sprintf("account-purge-request:%s:%d", id, generation), nil, "account.purge_requested", "user", id, string(payload), now); err != nil {
		return err
	}
	return tx.Commit()
}

const accountJobTargets = `((CASE ((payload)::jsonb #> '{userId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (payload)::jsonb #>> '{userId}' END)=$1
 OR (CASE ((payload)::jsonb #> '{purchaseId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (payload)::jsonb #>> '{purchaseId}' END) IN (SELECT id FROM purchases WHERE user_id=$2)
 OR (CASE ((payload)::jsonb #> '{accountId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (payload)::jsonb #>> '{accountId}' END) IN (SELECT id FROM emby_accounts WHERE user_id=$3)
 OR (CASE ((payload)::jsonb #> '{operationId}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (payload)::jsonb #>> '{operationId}' END) IN (SELECT id FROM provider_operations WHERE owner_user_id=$4 OR actor_user_id=$5))`

func cancelAccountQueuedJobsTx(ctx context.Context, tx *sql.Tx, id string) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM outbox_jobs WHERE status<>'processing' AND kind IN
 ('xnode_access_repair','xnode_apply_entitlement','xnode_sync_user','xnode_prepare_continuity','xnode_payg_reconcile','xnode_payg_recover',
 'emby_provision_account','provider_operation','user_notification') AND `+accountJobTargets, id, id, id, id, id)
	return err
}
