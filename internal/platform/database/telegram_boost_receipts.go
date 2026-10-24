package database

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// EnqueueBoostAppreciation atomically records a boost identity and its durable job.
func (s *Store) retryAttempt_EnqueueBoostAppreciation(ctx context.Context, item model.TelegramBoostAppreciation, now time.Time) (bool, error) {
	if item.ChatID >= 0 || strings.TrimSpace(item.BoostID) == "" {
		return false, ErrConflict
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO telegram_boost_receipts(chat_id,boost_id,created_at) VALUES(?,?,?)`, item.ChatID, item.BoostID, stamp(now))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if count == 0 {
		return false, nil
	}
	payload, err := json.Marshal(item)
	if err != nil {
		return false, err
	}
	if err := insertOutboxTx(ctx, tx, "telegram_boost_appreciation", string(payload), now, now); err != nil {
		return false, err
	}
	return true, tx.Commit()
}
