package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Unused reward credit has no account generation of its own. Mark it spent
// without a purchase so a rejoin cannot restore a forfeited operational benefit.
func forfeitAccountExtensionsTx(ctx context.Context, tx *sql.Tx, user string, now time.Time) error {
	var count, generation int64
	var hours string
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(COALESCE(hours,days*24)),0)::text,
 (SELECT account_generation FROM users WHERE id=$1) FROM activity_extension_credits
 WHERE user_id=$1 AND consumed_at IS NULL`, user).Scan(&count, &hours, &generation)
	if err != nil || count == 0 {
		return err
	}
	detail, err := json.Marshal(map[string]any{"generation": generation, "credits": count, "hours": hours})
	if err != nil {
		return err
	}
	if err = insertAuditTx(ctx, tx, fmt.Sprintf("account-purge-extensions:%s:%d", user, generation), nil,
		"account.forfeited_extension_credit", "user", user, string(detail), now); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE activity_extension_credits SET consumed_at=$2,
 consumed_by_purchase_id=NULL WHERE user_id=$1 AND consumed_at IS NULL`, user, stamp(now))
	return err
}
