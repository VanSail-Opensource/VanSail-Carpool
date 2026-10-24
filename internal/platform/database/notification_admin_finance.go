package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// AdjustAdminBalance atomically records the ledger, audit, and user message.
func (s *Store) AdjustAdminBalance(ctx context.Context, actorID, userID string, delta int64, referenceID, reason string,
	now time.Time) (model.LedgerEntry, error) {
	return s.adminBalanceChange(ctx, actorID, userID, delta, referenceID, reason, "admin_adjustment", "balance.adjust",
		"balance_adjustment", now)
}

// DeductAdminBalance atomically records an exact no-debt deduction and message.
func (s *Store) DeductAdminBalance(ctx context.Context, actorID, userID string, amount int64, referenceID, reason string,
	now time.Time) (model.LedgerEntry, error) {
	if amount <= 0 {
		return model.LedgerEntry{}, ErrInsufficientBalance
	}
	return s.adminBalanceChange(ctx, actorID, userID, -amount, referenceID, reason, "telegram_deduct",
		"telegram.balance_deduct", "balance_deduction", now)
}

func (s *Store) retryAttempt_adminBalanceChange(ctx context.Context, actorID, userID string, delta int64, referenceID, reason,
	ledgerKind, action, change string, now time.Time) (model.LedgerEntry, error) {
	reason = strings.TrimSpace(reason)
	if actorID == "" || userID == "" || delta == 0 || reason == "" || len(reason) > 500 {
		return model.LedgerEntry{}, errors.New("administrator balance change is invalid")
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
	var existingID, existingUserID, existingNote string
	var existingDelta int64
	if err := tx.QueryRowContext(ctx, `SELECT id,user_id,delta_txb_minor,note FROM ledger_entries WHERE kind=? AND reference_id=?`, ledgerKind, referenceID).
		Scan(&existingID, &existingUserID, &existingDelta, &existingNote); err == nil {
		if existingUserID != userID || existingDelta != delta || existingNote != reason {
			return model.LedgerEntry{}, ErrConflict
		}
		if err := tx.Commit(); err != nil {
			return model.LedgerEntry{}, err
		}
		return s.LedgerEntryByID(ctx, existingID)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return model.LedgerEntry{}, err
	}
	var balance int64
	if ledgerKind == "telegram_deduct" {
		balance, err = changeBalanceTx(ctx, tx, userID, delta, now)
	} else {
		balance, err = adjustBalanceTx(ctx, tx, userID, delta, now)
	}
	if err != nil {
		return model.LedgerEntry{}, err
	}
	entryID, err := insertLedgerTx(ctx, tx, userID, delta, balance, ledgerKind, referenceID, reason, now)
	if err != nil {
		return model.LedgerEntry{}, err
	}
	detail, err := json.Marshal(map[string]any{"deltaMinor": delta, "reason": reason, "ledgerEntryId": entryID})
	if err != nil {
		return model.LedgerEntry{}, err
	}
	auditID, err := ids.New()
	if err != nil {
		return model.LedgerEntry{}, err
	}
	if err := insertAuditTx(ctx, tx, auditID, &actorID, action, "user", userID, string(detail), now); err != nil {
		return model.LedgerEntry{}, err
	}
	_, err = s.insertUserNotificationTx(ctx, tx, "admin:"+entryID+":"+userID, userID, jobpayload.UserEventAdminUpdate, "",
		adminFinanceFacts(change, delta, balance, reason, now), now)
	if err != nil {
		return model.LedgerEntry{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.LedgerEntry{}, err
	}
	return s.LedgerEntryByID(ctx, entryID)
}

func adminFinanceFacts(change string, amount, balance int64, reason string, now time.Time) map[string]string {
	return map[string]string{
		notifications.FactChange: change, notifications.FactAmount: strconv.FormatInt(amount, 10),
		notifications.FactBalance: strconv.FormatInt(balance, 10), notifications.FactReason: reason,
		notifications.FactTime: now.UTC().Format(time.RFC3339Nano),
	}
}
