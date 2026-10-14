package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

// PublishRaffle queues the group announcement; entries remain closed until its ID is saved.
func (s *Store) PublishRaffle(ctx context.Context, id string, groupID int64, now time.Time) error {
	if groupID == 0 {
		return activity.ErrInvalidInput
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	draw, err := luckyDrawByID(ctx, tx, id, false)
	if err != nil {
		return err
	}
	draw.Prizes, err = luckyPrizes(ctx, tx, id, false)
	if err != nil {
		return err
	}
	if draw.Kind != "raffle" || draw.Status != "draft" {
		return ErrConflict
	}
	if err = draw.LuckyDrawInput.Validate(); err != nil {
		return err
	}
	if err = requireRaffleTriggersAvailableTx(ctx, tx, draw.LuckyDrawInput); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_lucky_draws SET status='publishing',group_chat_id=?,updated_at=? WHERE id=? AND status='draft'`, groupID, stamp(now), id); err != nil {
		if isUniqueConstraint(err) {
			return ErrConflict
		}
		return err
	}
	payload, _ := json.Marshal(map[string]any{"drawId": id})
	if err = insertOutboxTx(ctx, tx, "draw_telegram_publish", string(payload), now, now); err != nil {
		return err
	}
	return tx.Commit()
}

// ConfirmRafflePublished opens a publishing draft or schedules cleanup if the
// administrator cancelled while Telegram was sending the announcement.
func (s *Store) ConfirmRafflePublished(ctx context.Context, id string, messageID int64, now time.Time) error {
	if messageID <= 0 {
		return activity.ErrInvalidInput
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	draw, err := luckyDrawByID(ctx, tx, id, false)
	if err != nil {
		return err
	}
	if draw.Kind != "raffle" {
		return ErrConflict
	}
	if draw.AnnouncementMessageID != 0 {
		if draw.AnnouncementMessageID == messageID {
			return nil
		}
		return ErrConflict
	}
	status := "open"
	if draw.Status == "cancelled" {
		status = "cancelled"
	} else if draw.Status != "publishing" {
		return ErrConflict
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_lucky_draws SET status=?,announcement_message_id=?,updated_at=? WHERE id=?`,
		status, messageID, stamp(now), id); err != nil {
		return err
	}
	if status == "cancelled" {
		// A prior delete job may already be processing an empty message ID.
		// Include this ID so outbox deduplication retains the compensating cleanup.
		payload, marshalErr := json.Marshal(map[string]any{"drawId": id, "messageId": messageID})
		if marshalErr != nil {
			return marshalErr
		}
		if err = insertOutboxTx(ctx, tx, "draw_telegram_delete", string(payload), now, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Settling raffles still own their triggers: an eligibility refund may reopen them.
// Existing indexes only cover publishing/open, so enforce this in both write paths.
func requireRaffleTriggersAvailableTx(ctx context.Context, tx *sql.Tx, input activity.LuckyDrawInput) error {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_lucky_draws WHERE id<>? AND kind='raffle'
		AND status IN ('publishing','open','settling') AND (lower(keyword)=lower(?) OR lower(command)=lower(?))`,
		input.ID, input.Keyword, input.Command).Scan(&count)
	if err != nil {
		return err
	}
	if count != 0 {
		return ErrConflict
	}
	return nil
}
