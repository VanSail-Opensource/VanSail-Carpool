package database

import (
	"context"
	"database/sql"
	"time"
)

// AccountOperationPermitted rejects old lifecycle receipts before dispatch.
func (s *Store) AccountOperationPermitted(ctx context.Context, operationID string) (bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT u.account_status,u.last_purged_at,o.created_at FROM provider_operations o JOIN users u ON u.id=o.owner_user_id OR u.id=o.actor_user_id WHERE o.id=?`, operationID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var status, created string
		var cutoff sql.NullString
		if err = rows.Scan(&status, &cutoff, &created); err != nil {
			return false, err
		}
		if status != "active" {
			return false, nil
		}
		if cutoff.Valid {
			at, err := parseStamp(created)
			if err != nil {
				return false, err
			}
			limit, err := parseStamp(cutoff.String)
			if err != nil {
				return false, err
			}
			if !at.After(limit) {
				return false, nil
			}
		}
	}
	return true, rows.Err()
}

func accountOrderRetiredTx(ctx context.Context, tx *sql.Tx, userID string, createdAt time.Time) (bool, error) {
	var status string
	var cutoff sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT account_status,last_purged_at FROM users WHERE id=?`, userID).Scan(&status, &cutoff)
	if err != nil {
		return false, err
	}
	if status != "active" {
		return true, nil
	}
	if !cutoff.Valid {
		return false, nil
	}
	at, err := parseStamp(cutoff.String)
	return !createdAt.After(at), err
}
