package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"math"
	"strings"
	"time"
)

func balanceTx(ctx context.Context, tx *sql.Tx, userID string) (int64, error) {
	var balance int64
	if err := tx.QueryRowContext(ctx, `SELECT txb_minor FROM balances WHERE user_id=? /*pg:FOR UPDATE*/`, userID).Scan(&balance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("read TXB balance: %w", err)
	}
	return balance, nil
}

func checkedBalanceAddition(current, delta int64) (int64, error) {
	if (delta > 0 && current > math.MaxInt64-delta) || (delta < 0 && current < math.MinInt64-delta) {
		return 0, fmt.Errorf("TXB balance overflow")
	}
	return current + delta, nil
}

func setBalanceTx(ctx context.Context, tx *sql.Tx, userID string, current, next int64, now time.Time) error {
	result, err := tx.ExecContext(ctx, `UPDATE balances SET txb_minor=?,updated_at=? WHERE user_id=? AND txb_minor=? AND NOT EXISTS(SELECT 1 FROM users WHERE id=balances.user_id AND account_status='purged')`, next, stamp(now), userID, current)
	if err != nil {
		return fmt.Errorf("change TXB balance: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return rowsErr
	} else if affected != 1 {
		return ErrConflict
	}
	if current != next {
		return enqueuePAYGUserTx(ctx, tx, userID, now)
	}
	return nil
}

// adjustBalanceTx performs a signed balance change without silently allowing
// SQLite to promote an overflowing INTEGER expression to REAL. It is reserved
// for operations that may intentionally create debt, such as administrator
// adjustments and provider reversals.
func adjustBalanceTx(ctx context.Context, tx *sql.Tx, userID string, delta int64, now time.Time) (int64, error) {
	current, err := balanceTx(ctx, tx, userID)
	if err != nil {
		return 0, err
	}
	next, err := checkedBalanceAddition(current, delta)
	if err != nil {
		return 0, err
	}
	if err := setBalanceTx(ctx, tx, userID, current, next, now); err != nil {
		return 0, err
	}
	return next, nil
}

func changeBalanceTx(ctx context.Context, tx *sql.Tx, userID string, delta int64, now time.Time) (int64, error) {
	current, err := balanceTx(ctx, tx, userID)
	if err != nil {
		return 0, err
	}
	next, err := checkedBalanceAddition(current, delta)
	if err != nil {
		return 0, err
	}
	if delta < 0 && next < 0 {
		return 0, ErrInsufficientBalance
	}
	if err := setBalanceTx(ctx, tx, userID, current, next, now); err != nil {
		return 0, err
	}
	return next, nil
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableStamp(value *time.Time) any {
	if value == nil {
		return nil
	}
	return stamp(*value)
}

func nullableStringValue(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func int64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func isUniqueConstraint(err error) bool {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		return pg.Code == "23505"
	}
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}
