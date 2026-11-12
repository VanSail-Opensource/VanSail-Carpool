package database

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

// TelegramStarCursor retains the next unprocessed chronological history position.
// Anchor hashes the transaction immediately before NextOffset, including direction.
type TelegramStarCursor struct {
	BotID       int64
	Generation  int64
	NextOffset  int64
	Anchor      string
	AvailableAt time.Time
}

const telegramStarCursorColumns = `bot_id,generation,next_offset,anchor,available_at`

// TelegramStarReconciliationCursor reads a bot's recoverable history boundary.
func (s *Store) TelegramStarReconciliationCursor(ctx context.Context, botID int64) (TelegramStarCursor, error) {
	return scanTelegramStarCursor(s.db.QueryRowContext(ctx, `SELECT `+telegramStarCursorColumns+` FROM telegram_star_reconciliation WHERE bot_id=$1`, botID))
}

func scanTelegramStarCursor(row rowScanner) (TelegramStarCursor, error) {
	var cursor TelegramStarCursor
	var available string
	err := row.Scan(&cursor.BotID, &cursor.Generation, &cursor.NextOffset, &cursor.Anchor, &available)
	if errors.Is(err, sql.ErrNoRows) {
		return cursor, ErrNotFound
	}
	if err == nil {
		cursor.AvailableAt, err = parseStamp(available)
	}
	return cursor, err
}

// EnsureTelegramStarReconciliation resumes the current page after restart or
// exhausted delivery retries without moving its boundary or scheduled time.
func (s *Store) retryAttempt_EnsureTelegramStarReconciliation(ctx context.Context, botID int64, now time.Time) error {
	if botID <= 0 {
		return ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO telegram_star_reconciliation(bot_id,generation,next_offset,anchor,available_at,updated_at)
		VALUES($1,1,0,'',$2,$2) ON CONFLICT(bot_id) DO NOTHING`, botID, stamp(now)); err != nil {
		return err
	}
	cursor, err := scanTelegramStarCursor(tx.QueryRowContext(ctx, `SELECT `+telegramStarCursorColumns+` FROM telegram_star_reconciliation WHERE bot_id=$1 FOR UPDATE`, botID))
	if err != nil {
		return err
	}
	if err := enqueueTelegramStarPageTx(ctx, tx, cursor, now); err != nil {
		return err
	}
	return tx.Commit()
}

// AdvanceTelegramStarReconciliation commits a cursor and its continuation
// together. Replayed or superseded pages cannot move a later generation.
func (s *Store) retryAttempt_AdvanceTelegramStarReconciliation(ctx context.Context, expected TelegramStarCursor,
	nextOffset int64, anchor string, availableAt, now time.Time) error {
	if expected.BotID <= 0 || expected.Generation <= 0 || expected.Generation == math.MaxInt64 ||
		nextOffset < 0 || nextOffset > math.MaxInt32 || !validTelegramStarAnchor(nextOffset, anchor) {
		return ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE telegram_star_reconciliation SET generation=generation+1,next_offset=$1,anchor=$2,available_at=$3,updated_at=$4
		WHERE bot_id=$5 AND generation=$6 AND next_offset=$7 AND anchor=$8`, nextOffset, anchor, stamp(availableAt), stamp(now),
		expected.BotID, expected.Generation, expected.NextOffset, expected.Anchor)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed == 0 {
		return err
	}
	next := TelegramStarCursor{BotID: expected.BotID, Generation: expected.Generation + 1, NextOffset: nextOffset, Anchor: anchor, AvailableAt: availableAt}
	if err := enqueueTelegramStarPageTx(ctx, tx, next, now); err != nil {
		return err
	}
	return tx.Commit()
}

func enqueueTelegramStarPageTx(ctx context.Context, tx *sql.Tx, cursor TelegramStarCursor, now time.Time) error {
	payload, err := json.Marshal(jobpayload.StarsReconciliationPage{BotID: cursor.BotID, Generation: cursor.Generation})
	if err != nil {
		return err
	}
	availableAt := cursor.AvailableAt
	if availableAt.Before(now) {
		availableAt = now
	}
	return insertOutboxTx(ctx, tx, jobpayload.StarsReconciliationKind, string(payload), availableAt, now)
}

func validTelegramStarAnchor(offset int64, anchor string) bool {
	if offset == 0 {
		return anchor == ""
	}
	decoded, err := hex.DecodeString(anchor)
	return err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == anchor
}
