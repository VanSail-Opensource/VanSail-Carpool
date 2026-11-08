package database

import (
	"context"
	"database/sql"
)

// New intents share lifecycle locks until INSERT/queue commit. Exact prior
// receipts replay before this check and never create another external mutation.
// Some callers already hold owner/wallet locks; existing transaction deadlock
// retries cover cross-actor ordering without a global process lock.
func lockProviderOperationAccountsTx(ctx context.Context, tx *sql.Tx, actor, owner string) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,account_status FROM users WHERE id=$1 OR id=$2 ORDER BY id FOR SHARE`, actor, owner)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for rows.Next() {
		var id, status string
		if err = rows.Scan(&id, &status); err != nil {
			rows.Close()
			return err
		}
		if status != "active" {
			rows.Close()
			return ErrAccountUnavailable
		}
		seen[id] = true
	}
	if err = finishCompileRows(rows); err != nil {
		return err
	}
	if !seen[actor] || owner != "" && !seen[owner] {
		return ErrNotFound
	}
	return nil
}
