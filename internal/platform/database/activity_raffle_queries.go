package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

// RaffleTicketInfo contains the Telegram routing and current progress for a seat.
type RaffleTicketInfo struct {
	ID, DrawID, UserID string
	Status             string
	ChatID, MessageID  int64
	Receipt            activity.RaffleEntry
}

func (s *Store) RaffleTicketByID(ctx context.Context, id string) (RaffleTicketInfo, error) {
	var item RaffleTicketInfo
	err := s.db.QueryRowContext(ctx, `SELECT id,draw_id,user_id,chat_id,message_id,status FROM activity_raffle_tickets WHERE id=?`, id).
		Scan(&item.ID, &item.DrawID, &item.UserID, &item.ChatID, &item.MessageID, &item.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrNotFound
	}
	if err != nil {
		return item, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return item, err
	}
	defer func() { _ = tx.Rollback() }()
	item.Receipt, err = raffleEntryReceiptTx(ctx, tx, item.DrawID, item.UserID, 0)
	return item, err
}
func (s *Store) LuckyDrawByID(ctx context.Context, id string) (activity.LuckyDraw, error) {
	draw, err := luckyDrawByID(ctx, s.db, id, false)
	if err != nil {
		return draw, err
	}
	draw.Prizes, err = luckyPrizes(ctx, s.db, id, false)
	return draw, err
}
func (s *Store) LuckyDrawResultByID(ctx context.Context, id string) (activity.DrawResult, error) {
	return s.drawResultByID(ctx, id)
}
func (s *Store) RaffleResults(ctx context.Context, id string) ([]activity.DrawResult, error) {
	rows, err := s.db.QueryContext(ctx, drawResultSelect+` WHERE draw_id=? AND idempotency_key LIKE 'raffle:%' ORDER BY created_at,id`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	results := make([]activity.DrawResult, 0)
	for rows.Next() {
		result, scanErr := scanDrawResult(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		results = append(results, result)
	}
	return results, rows.Err()
}
func (s *Store) RaffleProgress(ctx context.Context, id string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_raffle_tickets WHERE draw_id=? AND status IN ('active','settled')`, id).Scan(&count)
	return count, err
}
func (s *Store) DrawDelivery(ctx context.Context, key string) (int64, bool, error) {
	var messageID int64
	err := s.db.QueryRowContext(ctx, `SELECT message_id FROM activity_draw_delivery WHERE delivery_key=?`, key).Scan(&messageID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return messageID, err == nil, err
}
func (s *Store) RecordDrawDelivery(ctx context.Context, key string, chatID, messageID int64, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO activity_draw_delivery(delivery_key,chat_id,message_id,created_at) VALUES(?,?,?,?)`,
		key, chatID, messageID, stamp(now))
	return err
}
