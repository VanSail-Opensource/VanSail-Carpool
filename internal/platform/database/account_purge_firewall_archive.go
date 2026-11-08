package database

import (
	"context"
	"database/sql"
	"time"
)

// Archive the original paid grant without retaining a user's IP address. These
// receipts are not a live firewall inventory and cannot re-enable an allowance.
func archiveAccountFirewallGrantsTx(ctx context.Context, tx *sql.Tx, user string, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,jsonb_build_object(
 'grantId',id,'idempotencyKey',idempotency_key,'priceMinor',price_minor::text,
 'hours',hours::text,'untilAt',until_at,'createdAt',created_at,'ledgerId',ledger_id,
 'kind',grant_kind,'issuedVersion',issued_version::text,'releasedVersion',released_version::text)::text
 FROM xnode_firewall_exemptions WHERE user_id=$1 ORDER BY id`, user)
	if err != nil {
		return err
	}
	type receipt struct{ id, detail string }
	var receipts []receipt
	for rows.Next() {
		var r receipt
		if err = rows.Scan(&r.id, &r.detail); err != nil {
			rows.Close()
			return err
		}
		receipts = append(receipts, r)
	}
	if err = finishCompileRows(rows); err != nil {
		return err
	}
	for _, r := range receipts {
		if err = insertAuditTx(ctx, tx, "account-purge-firewall:"+r.id, nil,
			"account.archived_firewall_purchase", "user", user, r.detail, now); err != nil {
			return err
		}
	}
	return nil
}
