package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

var (
	ErrHourlyCreditUnavailable = model.ErrHourlyCreditUnavailable
	ErrHourlyCreditExpired     = model.ErrHourlyCreditExpired
)

// All callbacks are private SQL-only mutations; rollback precedes any retry.
func hourlyCreditTransaction[T any](ctx context.Context, s *Store, action func(*sql.Tx) (T, error)) (T, error) {
	for retry := 0; ; retry++ {
		value, err := hourlyCreditAttempt(ctx, s.db, action)
		if !s.retryTransaction(ctx, err, retry) {
			return value, err
		}
	}
}

func hourlyCreditAttempt[T any](ctx context.Context, db *sql.DB, action func(*sql.Tx) (T, error)) (T, error) {
	var zero T
	// Each statement after the owner/agent locks must observe the earlier
	// transaction's committed reservation, even if database defaults differ.
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return zero, err
	}
	defer tx.Rollback()
	value, err := action(tx)
	if err != nil {
		if errors.Is(err, model.ErrHourlyCreditNotReserved) {
			if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
				return zero, rollbackErr
			}
		}
		return zero, err
	}
	if err = tx.Commit(); err != nil {
		return zero, err
	}
	return value, nil
}

func validHourlyCreditName(value string) bool {
	if value == "" || len(value) > 128 || strings.HasPrefix(value, "_xnode_") {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("._-", c)) {
			return false
		}
	}
	return true
}

func validHourlyCreditHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func validHourlyCreditDay(day string) bool {
	at, err := time.Parse(time.DateOnly, day)
	return err == nil && at.Format(time.DateOnly) == day
}

func validateHourlyCreditRequest(r model.HourlyCreditRequest) error {
	if !validHourlyCreditName(r.ReservationID) || !validHourlyCreditName(r.OwnerID) || r.AccountGeneration < 0 || !validHourlyCreditDay(r.DayUTC) || !validHourlyCreditName(r.NodeID) || !validHourlyCreditHex(r.AgentID, 32) || !validHourlyCreditHex(r.BootID, 32) || !validHourlyCreditHex(r.SessionID, 32) || r.ByteCeiling <= 0 || r.Kind != "free" && r.Kind != "extra" {
		return ErrConflict
	}
	return nil
}

func lockHourlyOwnerTx(ctx context.Context, tx *sql.Tx, owner string, generation int64) error {
	var status string
	var current int64
	err := tx.QueryRowContext(ctx, `SELECT account_status,account_generation FROM users WHERE id=$1 FOR NO KEY UPDATE`, owner).Scan(&status, &current)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err == nil && (status != "active" || current != generation) {
		return ErrAccountUnavailable
	}
	return err
}

func currentHourlyCreditDayTx(ctx context.Context, tx *sql.Tx, day string) error {
	var current bool
	if err := tx.QueryRowContext(ctx, `SELECT $1::date=(clock_timestamp() AT TIME ZONE 'UTC')::date`, day).Scan(&current); err != nil {
		return err
	}
	if !current {
		return ErrHourlyCreditExpired
	}
	return nil
}
