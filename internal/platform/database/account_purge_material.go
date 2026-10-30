package database

import (
	"context"
	"database/sql"
	"time"
)

func purgeAccountMaterialTx(ctx context.Context, tx *sql.Tx, id string, now time.Time) error {
	if err := cancelAccountQueuedJobsTx(ctx, tx, id); err != nil {
		return err
	}
	queries := []struct {
		sql  string
		args []any
	}{
		{`UPDATE provider_operation_items SET status='failed',error_code='ACCOUNT_PURGED',completed_at=?,updated_at=? WHERE status IN ('queued','processing','pending_review') AND operation_id IN (SELECT id FROM provider_operations WHERE actor_user_id=? OR owner_user_id=?)`, []any{stamp(now), stamp(now), id, id}},
		{`UPDATE provider_operations SET status='failed',error_code='ACCOUNT_PURGED',completed_at=?,updated_at=? WHERE status IN ('queued','processing','pending_review','partial') AND (actor_user_id=? OR owner_user_id=?)`, []any{stamp(now), stamp(now), id, id}},
		{`DELETE FROM telegram_pm_payloads WHERE operation_id IN (SELECT id FROM provider_operations WHERE actor_user_id=? OR owner_user_id=?)`, []any{id, id}},
		{`DELETE FROM telegram_pm_conversations WHERE user_id=?`, []any{id}},
		{`DELETE FROM emby_accounts WHERE user_id=?`, []any{id}},
		{`DELETE FROM user_notification_events WHERE user_id=?`, []any{id}},
		{`DELETE FROM user_preferences WHERE user_id=?`, []any{id}},
		{`DELETE FROM user_host_names WHERE user_id=?`, []any{id}},
		{`DELETE FROM admin_api_tokens WHERE user_id=?`, []any{id}},
		{`UPDATE users SET inviter_id=NULL WHERE inviter_id=(SELECT telegram_id FROM users WHERE id=?)`, []any{id}},
		{`DELETE FROM user_disabled_squads WHERE user_id=?`, []any{id}},
		{`DELETE FROM user_disabled_payg_squads WHERE user_id=?`, []any{id}},
		{`DELETE FROM xnode_member_diy WHERE user_id=?`, []any{id}},
		{`DELETE FROM xnode_customizations WHERE user_id=? OR proxy_user_id IN (SELECT proxy_user_id FROM xnode_payg_accounts WHERE user_id=?)`, []any{id, id}},
		{`DELETE FROM panel_entry_verification WHERE telegram_id=(SELECT telegram_id FROM users WHERE id=?)`, []any{id}},
		{`INSERT INTO coupon_grant_discards(grant_id,user_id,discarded_at) SELECT id,user_id,? FROM coupon_grants WHERE user_id=? AND status='active' ON CONFLICT(grant_id) DO NOTHING`, []any{stamp(now), id}},
		{`UPDATE xnode_identities SET credentials_cipher='',subscription_cipher='',subscription_hash=NULL,current_period_id=NULL,access_disabled=1 WHERE user_id=? OR proxy_user_id IN (SELECT proxy_user_id FROM xnode_payg_accounts WHERE user_id=?)`, []any{id, id}},
		{`UPDATE xnode_payg_accounts SET credit_bytes=0,credit_started_at=NULL WHERE user_id=?`, []any{id}},
	}
	for _, q := range queries {
		if _, err := tx.ExecContext(ctx, q.sql, q.args...); err != nil {
			return err
		}
	}
	return nil
}
