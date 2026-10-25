package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"time"
)

// Balance returns the current TXB amount.
func (s *Store) Balance(ctx context.Context, userID string) (model.Money, error) {
	var balance int64
	if err := s.db.QueryRowContext(ctx, `SELECT txb_minor FROM balances WHERE user_id=?`, userID).Scan(&balance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.TXBMoney(0), nil
		}
		return model.Money{}, fmt.Errorf("read balance: %w", err)
	}
	return model.TXBMoney(balance), nil
}

// AdjustBalance appends an immutable administrator-authored ledger entry.
func (s *Store) retryAttempt_AdjustBalance(ctx context.Context, userID string, delta int64, referenceID, note string, now time.Time) (model.LedgerEntry, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.LedgerEntry{}, err
	}
	defer func() { _ = tx.Rollback() }()
	balance, err := adjustBalanceTx(ctx, tx, userID, delta, now)
	if err != nil {
		return model.LedgerEntry{}, fmt.Errorf("adjust balance: %w", err)
	}
	entryID, err := insertLedgerTx(ctx, tx, userID, delta, balance, "admin_adjustment", referenceID, note, now)
	if err != nil {
		return model.LedgerEntry{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.LedgerEntry{}, err
	}
	return s.LedgerEntryByID(ctx, entryID)
}

// DeductBalance appends an immutable administrator-authored debit without allowing debt.
func (s *Store) retryAttempt_DeductBalance(ctx context.Context, userID string, amount int64, referenceID, note string, now time.Time) (model.LedgerEntry, error) {
	if amount <= 0 {
		return model.LedgerEntry{}, ErrInsufficientBalance
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.LedgerEntry{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var existingID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM ledger_entries WHERE kind='telegram_deduct' AND reference_id=? LIMIT 1`, referenceID).Scan(&existingID); err == nil {
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

func insertLedgerTx(ctx context.Context, tx *sql.Tx, userID string, delta, balance int64, kind, referenceID, note string, now time.Time) (string, error) {
	id, err := ids.New()
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ledger_entries(id,user_id,delta_txb_minor,balance_after,kind,reference_id,note,created_at) VALUES(?,?,?,?,?,?,?,?)`, id, userID, delta, balance, kind, referenceID, note, stamp(now))
	if err != nil {
		return "", fmt.Errorf("append ledger: %w", err)
	}
	if delta < 0 && memberCommerceKind(kind) {
		if _, err = tx.ExecContext(ctx, `INSERT INTO user_preferences(user_id,show_around_tx) VALUES(?,1)
			ON CONFLICT(user_id) DO UPDATE SET show_around_tx=1`, userID); err != nil {
			return "", fmt.Errorf("enable member community entrance: %w", err)
		}
	}
	return id, nil
}

func memberCommerceKind(kind string) bool {
	switch kind {
	case "purchase_debit", "purchase_addon_debit", "automatic_renewal", "payg_gb":
		return true
	}
	return false
}

// LedgerEntryByID returns one immutable ledger row.
func (s *Store) LedgerEntryByID(ctx context.Context, id string) (model.LedgerEntry, error) {
	return scanLedger(s.db.QueryRowContext(ctx, ledgerSelect+` WHERE id=?`, id))
}

// ListLedger returns newest entries first.
func (s *Store) ListLedger(ctx context.Context, userID string, limit int) ([]model.LedgerEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, ledgerSelect+` WHERE user_id=? ORDER BY created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list ledger: %w", err)
	}
	defer func() { _ = rows.Close() }()
	entries := make([]model.LedgerEntry, 0)
	for rows.Next() {
		entry, err := scanLedger(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

const ledgerSelect = `SELECT id,delta_txb_minor,balance_after,kind,reference_id,note,created_at FROM ledger_entries`

func scanLedger(row rowScanner) (model.LedgerEntry, error) {
	var entry model.LedgerEntry
	var created string
	if err := row.Scan(&entry.ID, &entry.DeltaTXBMinor, &entry.BalanceAfterRaw, &entry.Kind, &entry.ReferenceID, &entry.Note, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.LedgerEntry{}, ErrNotFound
		}
		return model.LedgerEntry{}, err
	}
	entry.Delta = model.TXBMoney(entry.DeltaTXBMinor)
	entry.BalanceAfter = model.TXBMoney(entry.BalanceAfterRaw)
	var err error
	entry.CreatedAt, err = parseStamp(created)
	return entry, err
}
