package database

import (
	"context"
	"database/sql"
	"time"
)

func shiftRefundQueueTx(ctx context.Context, tx *sql.Tx, userID string, firstFrom, targetFrom, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,valid_from,valid_until FROM purchases
		WHERE user_id=$1 AND status='queued' AND `+purchaseTimestampOrderSQL("valid_from")+`>=$2
		ORDER BY `+purchaseTimestampOrderSQL("valid_from")+`,`+purchaseTimestampOrderSQL("created_at")+`,id`, userID, purchaseTimestampOrder(firstFrom))
	if err != nil {
		return err
	}
	type queuedTerm struct{ id, from, until string }
	terms := make([]queuedTerm, 0)
	for rows.Next() {
		var term queuedTerm
		if err := rows.Scan(&term.id, &term.from, &term.until); err != nil {
			_ = rows.Close()
			return err
		}
		terms = append(terms, term)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for index, term := range terms {
		from, err := parseStamp(term.from)
		if err != nil {
			return err
		}
		until, err := parseStamp(term.until)
		if err != nil {
			return err
		}
		if !until.After(from) {
			return ErrConflict
		}
		shiftedFrom, err := rebasePurchaseTimestamp(from, firstFrom, targetFrom)
		if err != nil {
			return err
		}
		shiftedUntil, err := rebasePurchaseTimestamp(until, firstFrom, targetFrom)
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `UPDATE purchases SET valid_from=$1,valid_until=$2,updated_at=$3 WHERE id=$4 AND status='queued'`,
			stamp(shiftedFrom), stamp(shiftedUntil), stamp(now), term.id)
		if err != nil {
			return err
		}
		if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
			return rowsErr
		} else if affected != 1 {
			return ErrConflict
		}
		if index > 0 {
			if err := retirePurchaseContinuityTx(ctx, tx, term.id, now); err != nil {
				return err
			}
		}
	}
	return nil
}

// rebasePurchaseTimestamp preserves exact timeline offsets beyond time.Duration's
// roughly 292-year range. Supported RFC3339/database years keep second arithmetic
// within int64; the result must remain representable before any caller commits.
func rebasePurchaseTimestamp(at, firstFrom, targetFrom time.Time) (time.Time, error) {
	for _, value := range []time.Time{at, firstFrom, targetFrom} {
		if year := value.UTC().Year(); year < 1 || year > 9999 {
			return time.Time{}, ErrConflict
		}
	}
	seconds := at.Unix() + targetFrom.Unix() - firstFrom.Unix()
	nanos := int64(at.Nanosecond()) + int64(targetFrom.Nanosecond()) - int64(firstFrom.Nanosecond())
	result := time.Unix(seconds, nanos).UTC()
	if year := result.Year(); year < 1 || year > 9999 {
		return time.Time{}, ErrConflict
	}
	return result, nil
}
