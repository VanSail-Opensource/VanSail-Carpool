package database

import (
	"context"
	"database/sql"
	"time"
)

const accountNativePrincipals = `SELECT proxy_user_id FROM xnode_identities WHERE user_id=$1
 UNION SELECT proxy_user_id FROM xnode_payg_accounts WHERE user_id=$1
 UNION SELECT proxy_user_id FROM hourly_enrollments WHERE owner_id=$1`

// Call only after admission, command-outcome, period, byte-credit and scoped IP
// barriers. Accounting subjects and immutable financial receipts are retained.
func purgeAccountOperationalTx(ctx context.Context, tx *sql.Tx, user string, now time.Time) error {
	if err := forfeitAccountExtensionsTx(ctx, tx, user, now); err != nil {
		return err
	}
	if err := archiveAccountFirewallGrantsTx(ctx, tx, user, now); err != nil {
		return err
	}
	queries := []string{
		`DELETE FROM sessions WHERE user_id=$1`,
		`DELETE FROM browser_access_links WHERE user_id=$1`,
		`DELETE FROM hourly_account_preferences WHERE owner_id=$1`,
		`DELETE FROM xnode_routing_plans WHERE proxy_user_id IN(` + accountNativePrincipals + `)`,
		`DELETE FROM xnode_service_bindings WHERE proxy_user_id IN(` + accountNativePrincipals + `)`,
		`DELETE FROM xnode_customizations WHERE user_id=$1 OR proxy_user_id IN(` + accountNativePrincipals + `)`,
		`DELETE FROM connection_scans WHERE user_id=$1`,
		`DELETE FROM connection_ip_blocks WHERE user_id=$1`,
		`DELETE FROM xnode_firewall_exemptions WHERE user_id=$1`,
		`DELETE FROM user_notification_events WHERE user_id=$1`,
		`DELETE FROM activity_group_message_events WHERE user_id=$1`,
		`DELETE FROM activity_group_message_windows WHERE user_id=$1`,
		`DELETE FROM xnode_usage_minutes WHERE owner_id=$1`,
		`DELETE FROM xnode_payg_node_minutes WHERE user_id=$1`,
		`DELETE FROM xnode_payg_minutes WHERE user_id=$1`,
		`DELETE FROM account_cleanup_checks WHERE user_id=$1`,
		`UPDATE xnode_identities SET credentials_cipher='',subscription_cipher='',subscription_hash=NULL,
 current_period_id=NULL,access_disabled=1,first_synced_at=NULL WHERE proxy_user_id IN(` + accountNativePrincipals + `)`,
	}
	for _, query := range queries {
		if _, err := tx.ExecContext(ctx, query, user); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `UPDATE hourly_enrollments SET status='closed',disabled_at=COALESCE(disabled_at,$2),
 updated_at=$2 WHERE owner_id=$1 AND status<>'closed'`, user, now.UTC())
	return err
}
