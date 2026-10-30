package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// Keep the forfeited prepaid quantity beside immutable charge evidence.
func archiveAccountCreditTx(ctx context.Context, tx *sql.Tx, user string, generation int64, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT proxy_user_id,credit_bytes,credit_started_at FROM xnode_payg_accounts WHERE user_id=? AND credit_bytes>0`, user)
	if err != nil {
		return err
	}
	credits := []map[string]any{}
	for rows.Next() {
		var proxy, bytes int64
		var started sql.NullString
		if err = rows.Scan(&proxy, &bytes, &started); err != nil {
			rows.Close()
			return err
		}
		credits = append(credits, map[string]any{"proxyUserId": proxy, "bytes": bytes, "startedAt": started.String})
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(credits) == 0 {
		return err
	}
	detail, err := json.Marshal(map[string]any{"generation": generation, "credits": credits})
	if err != nil {
		return err
	}
	return insertAuditTx(ctx, tx, "account-purge-credit:"+user+":"+stamp(now), nil, "account.forfeited_credit", "user", user, string(detail), now)
}
