package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// Historical unaudited debits only seed wallet/concurrency regression fixtures.
// Production administrator debits use the actor-bound DeductAdminBalance.
func (s *Store) DeductBalance(ctx context.Context, userID string, amount int64, referenceID, note string, now time.Time) (model.LedgerEntry, error) {
	for retryIndex := 0; ; retryIndex++ {
		r0, r1 := s.retryAttempt_DeductBalance(ctx, userID, amount, referenceID, note, now)
		if !s.retryTransaction(ctx, r1, retryIndex) {
			return r0, r1
		}
	}
}

// DeductBalance appends an immutable administrator-authored debit without allowing debt.
func (s *Store) retryAttempt_DeductBalance(ctx context.Context, userID string, amount int64, referenceID, note string, now time.Time) (model.LedgerEntry, error) {
	if amount <= 0 {
		return model.LedgerEntry{}, ErrInsufficientBalance
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.LedgerEntry{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var existingID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM ledger_entries WHERE kind='telegram_deduct' AND reference_id=$1 LIMIT 1`, referenceID).Scan(&existingID); err == nil {
		_ = tx.Rollback()
		return s.LedgerEntryByID(ctx, existingID)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return model.LedgerEntry{}, err
	}
	balance, err := changeBalanceTx(ctx, tx, userID, -amount, now.UTC())
	if err != nil {
		return model.LedgerEntry{}, fmt.Errorf("deduct balance: %w", err)
	}
	entryID, err := insertLedgerTx(ctx, tx, userID, -amount, balance, "telegram_deduct", referenceID, note, now.UTC())
	if err != nil {
		return model.LedgerEntry{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.LedgerEntry{}, err
	}
	return s.LedgerEntryByID(ctx, entryID)
}
