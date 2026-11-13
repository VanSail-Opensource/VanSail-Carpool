package database

import (
	"context"
	"database/sql"
	"time"
)

// Shift only actual successors. Text ordering can omit a fractional successor
// after a whole-second boundary or move an earlier whole-second term.
func shiftExtensionQueueTx(ctx context.Context, tx *sql.Tx, userID, excludedID string, boundary time.Time, hours int, now time.Time) error {
	lower, _ := purchaseWindowCandidateBounds(boundary)
	rows, err := tx.QueryContext(ctx, `SELECT id,valid_from,valid_until FROM purchases
  WHERE user_id=$1 AND id<>$2 AND status='queued' AND valid_from>=$3 ORDER BY id`, userID, excludedID, lower)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type queuedTerm struct {
		id          string
		from, until time.Time
	}
	terms := make([]queuedTerm, 0)
	for rows.Next() {
		var term queuedTerm
		var from, until string
		if err := rows.Scan(&term.id, &from, &until); err != nil {
			return err
		}
		term.from, err = parseStamp(from)
		if err != nil {
			return err
		}
		term.until, err = parseStamp(until)
		if err != nil {
			return err
		}
		if !term.from.Before(boundary) {
			terms = append(terms, term)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, term := range terms {
		from, err := addSubscriptionHours(term.from, hours)
		if err != nil {
			return err
		}
		until, err := addSubscriptionHours(term.until, hours)
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `UPDATE purchases SET valid_from=$1,valid_until=$2,updated_at=$3 WHERE id=$4 AND status='queued'`, stamp(from), stamp(until), stamp(now), term.id)
		if err != nil {
			return err
		}
		if n, err := result.RowsAffected(); err != nil {
			return err
		} else if n != 1 {
			return ErrConflict
		}
	}
	return nil
}
