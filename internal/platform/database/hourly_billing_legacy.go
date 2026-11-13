package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrHourlyLegacyConflict = errors.New("hourly and legacy benefits cannot overlap")

// Hourly activation and legacy checkout share owner then wallet locks. Keeping
// that order also linearizes account closure with both forms of admission.
func lockHourlyLegacyPurchaseTx(ctx context.Context, tx *sql.Tx, owner string) error {
	var status string
	err := tx.QueryRowContext(ctx, `SELECT account_status FROM users WHERE id=$1 FOR NO KEY UPDATE`, owner).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "active" {
		return ErrAccountUnavailable
	}
	return lockMemberPurchaseTx(ctx, tx, owner)
}

// Call after immutable purchase replays, so a later hourly subscription cannot
// make an already accepted legacy checkout lose its receipt.
func rejectHourlyLegacyPurchaseTx(ctx context.Context, tx *sql.Tx, owner string) error {
	var found bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM hourly_enrollments
		WHERE owner_id=$1 AND account_generation=(SELECT account_generation FROM users WHERE id=$1)
		AND status IN ('preparing','active'))`, owner).Scan(&found)
	if err == nil && found {
		return ErrHourlyLegacyConflict
	}
	return err
}

func rejectLegacyHourlyActivationTx(ctx context.Context, tx *sql.Tx, owner string, at time.Time) error {
	var found bool
	// An accepted zero-price/coupon purchase is still a purchased benefit.
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE user_id=$1
		AND status IN ('active','activating','queued') AND `+purchaseTimestampOrderSQL("valid_until")+`>$2)`, owner, purchaseTimestampOrder(at)).Scan(&found)
	if err == nil && found {
		return ErrHourlyLegacyConflict
	}
	return err
}
