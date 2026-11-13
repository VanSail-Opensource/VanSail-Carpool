package database

import (
	"context"
	"database/sql"
	"time"
)

func shiftAdminSubscriptionTx(ctx context.Context, tx *sql.Tx, target adminBulkTarget, durationMinutes int, now time.Time) error {
	var validUntilRaw string
	if err := tx.QueryRowContext(ctx, `SELECT valid_until FROM purchases WHERE id=$1 AND user_id=$2 AND status='active'
		AND `+purchaseTimestampOrderSQL("valid_from")+`<=$3 AND `+purchaseTimestampOrderSQL("valid_until")+`>$4`, target.PurchaseID, target.UserID, purchaseTimestampOrder(now), purchaseTimestampOrder(now)).Scan(&validUntilRaw); err != nil {
		return err
	}
	validUntil, err := parseStamp(validUntilRaw)
	if err != nil {
		return err
	}
	shiftedUntil, err := addSubscriptionMinutes(validUntil, durationMinutes)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET valid_until=$1,updated_at=$2 WHERE id=$3 AND status='active'`,
		stamp(shiftedUntil), stamp(now), target.PurchaseID)
	if err != nil {
		return err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return rowsErr
		}
		return ErrConflict
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,valid_from,valid_until FROM purchases
		WHERE user_id=$1 AND status='queued' ORDER BY valid_from,id`, target.UserID)
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
	_ = rows.Close()
	for _, term := range terms {
		from, err := parseStamp(term.from)
		if err != nil {
			return err
		}
		until, err := parseStamp(term.until)
		if err != nil {
			return err
		}
		shiftedFrom, err := addSubscriptionMinutes(from, durationMinutes)
		if err != nil {
			return err
		}
		shiftedEnd, err := addSubscriptionMinutes(until, durationMinutes)
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `UPDATE purchases SET valid_from=$1,valid_until=$2,updated_at=$3
			WHERE id=$4 AND status='queued'`, stamp(shiftedFrom), stamp(shiftedEnd), stamp(now), term.id)
		if err != nil {
			return err
		}
		if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
			if rowsErr != nil {
				return rowsErr
			}
			return ErrConflict
		}
		if err := retirePurchaseContinuityTx(ctx, tx, term.id, now); err != nil {
			return err
		}
	}
	return nil
}

func addSubscriptionMinutes(value time.Time, minutes int) (time.Time, error) {
	if minutes < 1 || minutes > 3650*24*60 || value.UTC().Year() < 1 || value.UTC().Year() > 9999 {
		return time.Time{}, ErrConflict
	}
	shifted := value.Add(time.Duration(minutes) * time.Minute)
	if shifted.UTC().Year() > 9999 || !shifted.After(value) {
		return time.Time{}, ErrConflict
	}
	return shifted, nil
}
