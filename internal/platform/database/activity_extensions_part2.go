package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"math"
	"math/big"
	"time"
)

func applyPendingExtensionsToActivationTx(ctx context.Context, tx *sql.Tx, purchaseID string, now time.Time) error {
	var userID, validUntilRaw string
	if err := tx.QueryRowContext(ctx, `SELECT user_id,valid_until FROM purchases WHERE id=$1 AND status='activating'`, purchaseID).Scan(&userID, &validUntilRaw); err != nil {
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
	result, err := tx.ExecContext(ctx, `UPDATE purchases SET valid_until=$1,updated_at=$2 WHERE id=$3 AND status='activating'`, stamp(shiftedUntil), stamp(now), purchaseID)
	if err != nil {
		return err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return rowsErr
	} else if affected != 1 {
		return ErrConflict
	}
	return shiftExtensionQueueTx(ctx, tx, userID, purchaseID, validUntil, hours, now)
}

func addSubscriptionHours(value time.Time, hours int) (time.Time, error) {
	if hours <= 0 || int64(hours) > math.MaxInt64/int64(time.Hour) {
		return time.Time{}, activity.ErrInvalidInput
	}
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
