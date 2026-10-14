package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// JoinRaffle atomically sells one seat for one Telegram group message.
func (s *Store) JoinRaffle(ctx context.Context, userID string, chatID, messageID int64, text string, now time.Time) (activity.RaffleEntry, bool, error) {
	token, isCommand := raffleToken(text)
	if token == "" || chatID == 0 || messageID <= 0 {
		return activity.RaffleEntry{}, false, nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return activity.RaffleEntry{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	var replayDraw, replayUser string
	err = tx.QueryRowContext(ctx, `SELECT draw_id,user_id FROM activity_raffle_tickets WHERE chat_id=? AND message_id=?`, chatID, messageID).Scan(&replayDraw, &replayUser)
	if err == nil {
		if replayUser != userID {
			return activity.RaffleEntry{}, true, ErrConflict
		}
		receipt, readErr := raffleEntryReceiptTx(ctx, tx, replayDraw, userID, 0)
		receipt.Replayed = true
		return receipt, true, readErr
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return activity.RaffleEntry{}, false, err
	}
	column := "keyword"
	if isCommand {
		column = "command"
	}
	var drawID string
	query := `SELECT id FROM activity_lucky_draws WHERE kind='raffle' AND status='open' AND group_chat_id=? AND lower(` + column + `)=? LIMIT 1`
	err = tx.QueryRowContext(ctx, query, chatID, token).Scan(&drawID)
	if errors.Is(err, sql.ErrNoRows) {
		return activity.RaffleEntry{}, false, nil
	}
	if err != nil {
		return activity.RaffleEntry{}, false, err
	}
	draw, err := luckyDrawByID(ctx, tx, drawID, false)
	if err != nil {
		return activity.RaffleEntry{}, true, err
	}
	var onboarding string
	if err = tx.QueryRowContext(ctx, `SELECT onboarding_state FROM users WHERE id=?`, userID).Scan(&onboarding); err != nil {
		return activity.RaffleEntry{}, true, err
	}
	if onboarding != "complete" {
		return activity.RaffleEntry{}, true, ErrConflict
	}
	draw.Prizes, err = luckyPrizes(ctx, tx, drawID, false)
	if err != nil {
		return activity.RaffleEntry{}, true, err
	}
	if err = eligibleDrawParticipantTx(ctx, tx, userID, draw, now); err != nil {
		return activity.RaffleEntry{}, true, err
	}
	var existingSeats int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_raffle_tickets WHERE draw_id=? AND user_id=? AND status='active'`,
		drawID, userID).Scan(&existingSeats); err != nil {
		return activity.RaffleEntry{}, true, err
	}
	if err = raffleTrafficCoverageTx(ctx, tx, userID, draw, existingSeats+1, now); err != nil {
		return activity.RaffleEntry{}, true, err
	}
	reserve := draw.MaximumPrizeDeduction()
	balance, err := balanceTx(ctx, tx, userID)
	if err != nil {
		return activity.RaffleEntry{}, true, err
	}
	if balance < draw.FeeMinor || reserve > balance-draw.FeeMinor {
		return activity.RaffleEntry{}, true, ErrInsufficientBalance
	}
	ticketID, err := ids.New()
	if err != nil {
		return activity.RaffleEntry{}, true, err
	}
	feeBalance, err := changeBalanceTx(ctx, tx, userID, -draw.FeeMinor, now)
	if err != nil {
		return activity.RaffleEntry{}, true, err
	}
	if _, err = insertLedgerTx(ctx, tx, userID, -draw.FeeMinor, feeBalance, "activity_raffle_fee", ticketID, draw.Name, now); err != nil {
		return activity.RaffleEntry{}, true, err
	}
	if reserve > 0 {
		held, holdErr := changeBalanceTx(ctx, tx, userID, -reserve, now)
		if holdErr != nil {
			return activity.RaffleEntry{}, true, holdErr
		}
		if _, err = insertLedgerTx(ctx, tx, userID, -reserve, held, "activity_raffle_reserve", ticketID, draw.Name, now); err != nil {
			return activity.RaffleEntry{}, true, err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_raffle_tickets
		(id,draw_id,user_id,chat_id,message_id,fee_minor,reserve_minor,configuration_revision,status,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,'active',?,?)`, ticketID, drawID, userID, chatID, messageID, draw.FeeMinor, reserve, draw.Revision, stamp(now), stamp(now))
	if err != nil {
		return activity.RaffleEntry{}, true, err
	}
	receipt, err := raffleEntryReceiptTx(ctx, tx, drawID, userID, draw.FeeMinor)
	if err != nil {
		return activity.RaffleEntry{}, true, err
	}
	payload, _ := json.Marshal(map[string]any{"ticketId": ticketID})
	if err = insertOutboxTx(ctx, tx, "draw_raffle_reply", string(payload), now, now); err != nil {
		return activity.RaffleEntry{}, true, err
	}
	payload, _ = json.Marshal(map[string]any{"drawId": drawID, "revision": draw.Revision})
	if err = insertOutboxTx(ctx, tx, "draw_telegram_update", string(payload), now, now); err != nil {
		return activity.RaffleEntry{}, true, err
	}
	if receipt.Seats == draw.Threshold {
		result, updateErr := tx.ExecContext(ctx, `UPDATE activity_lucky_draws SET status='settling',updated_at=? WHERE id=? AND status='open'`, stamp(now), drawID)
		if updateErr != nil {
			return activity.RaffleEntry{}, true, updateErr
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return activity.RaffleEntry{}, true, ErrConflict
		}
		if err = insertOutboxTx(ctx, tx, "draw_raffle_settle", string(payload), now, now); err != nil {
			return activity.RaffleEntry{}, true, err
		}
	}
	if err = tx.Commit(); err != nil {
		return activity.RaffleEntry{}, true, fmt.Errorf("commit raffle seat: %w", err)
	}
	return receipt, true, nil
}

func raffleToken(text string) (string, bool) {
	value := strings.TrimSpace(text)
	if value == "" {
		return "", false
	}
	if !strings.HasPrefix(value, "/") {
		return strings.ToLower(value), false
	}
	fields := strings.Fields(value)
	if len(fields) != 1 {
		return "", true
	}
	command := strings.TrimPrefix(strings.ToLower(fields[0]), "/")
	if strings.ContainsRune(command, '@') {
		return "", true
	}
	return command, true
}

func raffleEntryReceiptTx(ctx context.Context, tx *sql.Tx, drawID, userID string, fee int64) (activity.RaffleEntry, error) {
	receipt := activity.RaffleEntry{DrawID: drawID, UserID: userID, FeeMinor: fee}
	err := tx.QueryRowContext(ctx, `SELECT threshold,
		(SELECT COUNT(*) FROM activity_raffle_tickets WHERE draw_id=? AND status IN ('active','settled')),
		(SELECT COUNT(*) FROM activity_raffle_tickets WHERE draw_id=? AND user_id=? AND status IN ('active','settled'))
  FROM activity_lucky_draws WHERE id=?`, drawID, drawID, userID, drawID).Scan(&receipt.Threshold, &receipt.Seats, &receipt.UserSeats)
	return receipt, err
}
