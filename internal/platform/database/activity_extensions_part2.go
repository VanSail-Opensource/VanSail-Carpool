package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"math/big"
	"time"
)

func applyPendingExtensionsToActivationTx(ctx context.Context, tx *sql.Tx, purchaseID string, now time.Time) error {
	var userID, validUntilRaw string
	if err := tx.QueryRowContext(ctx, `SELECT user_id,valid_until FROM purchases WHERE id=? AND status='activating'`, purchaseID).Scan(&userID, &validUntilRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrConflict
		}
		return err
	}
	hours, err := consumePendingExtensionsTx(ctx, tx, userID, purchaseID, now)
	if err != nil || hours == 0 {
		return err
	}
	validUntil, err := parseStamp(validUntilRaw)
	if err != nil {
		return err
	}
	shiftedUntil, err := addSubscriptionHours(validUntil, hours)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET valid_until=?,updated_at=? WHERE id=? AND status='activating'`, stamp(shiftedUntil), stamp(now), purchaseID)
	if err != nil {
		return err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return rowsErr
	} else if affected != 1 {
		return ErrConflict
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,valid_from,valid_until FROM purchases
		WHERE user_id=? AND id<>? AND status='queued' AND valid_from>=? ORDER BY valid_from,id`, userID, purchaseID, validUntilRaw)
	if err != nil {
		return err
	}
	type queuedTerm struct{ id, from, until string }
	queued := make([]queuedTerm, 0)
	for rows.Next() {
		var term queuedTerm
		if err := rows.Scan(&term.id, &term.from, &term.until); err != nil {
			_ = rows.Close()
			return err
		}
		queued = append(queued, term)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()
	for _, term := range queued {
		from, parseErr := parseStamp(term.from)
		if parseErr != nil {
			return parseErr
		}
		until, parseErr := parseStamp(term.until)
		if parseErr != nil {
			return parseErr
		}
		shiftedFrom, shiftErr := addSubscriptionHours(from, hours)
		if shiftErr != nil {
			return shiftErr
		}
		shiftedUntil, shiftErr := addSubscriptionHours(until, hours)
		if shiftErr != nil {
			return shiftErr
		}
		result, updateErr := tx.ExecContext(ctx, `UPDATE purchases SET valid_from=?,valid_until=?,updated_at=? WHERE id=? AND status='queued'`, stamp(shiftedFrom), stamp(shiftedUntil), stamp(now), term.id)
		if updateErr != nil {
			return updateErr
		}
		if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
			return rowsErr
		} else if affected != 1 {
			return ErrConflict
		}
	}
	return nil
}

func addSubscriptionHours(value time.Time, hours int) (time.Time, error) {
	shifted := value.Add(time.Duration(hours) * time.Hour)
	if shifted.Year() < 1 || shifted.Year() > 9999 || !shifted.After(value) {
		return time.Time{}, activity.ErrInvalidInput
	}
	return shifted, nil
}

func fixedMultiplyFloor(left, right, divisor int64) (int64, error) {
	if left < 0 || right < 0 || divisor <= 0 {
		return 0, activity.ErrInvalidInput
	}
	value := new(big.Int).Mul(big.NewInt(left), big.NewInt(right))
	value.Quo(value, big.NewInt(divisor))
	if !value.IsInt64() {
		return 0, activity.ErrInvalidInput
	}
	return value.Int64(), nil
}
