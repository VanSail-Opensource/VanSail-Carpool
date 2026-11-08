package database

import (
	"context"
	"database/sql"
	"errors"
)

// Legacy Emby creation used its account marker independently of the provider
// receipt. Serialize that marker before calling an intent safely unattempted.
func lockAccountPurgeEmbyTx(ctx context.Context, tx *sql.Tx, owner string) error {
	var id string
	// Setup/refund paths lock the wallet before the account. Keep that order
	// even when this phase only inspects the separate remote-attempt marker.
	err := tx.QueryRowContext(ctx, `SELECT user_id FROM balances WHERE user_id=$1 FOR UPDATE`, owner).Scan(&id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	err = tx.QueryRowContext(ctx, `SELECT id FROM emby_accounts WHERE user_id=$1 FOR UPDATE`, owner).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}
