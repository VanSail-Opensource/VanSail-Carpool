package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func activityGameByID(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, gameID string, enabledOnly bool) (activity.Game, error) {
	query := activityGameSelect + ` WHERE id=?`
	if enabledOnly {
		query += ` AND enabled=1`
	}
	return scanActivityGame(queryer.QueryRowContext(ctx, query, gameID))
}

func scanActivityGame(row rowScanner) (activity.Game, error) {
	var game activity.Game
	var enabled int
	var created, updated string
	if err := row.Scan(&game.ID, &game.Name, &game.Icon, &game.Description, &enabled, &game.WinChanceBPS, &game.MinimumStakeMinor, &game.MaximumStakeMinor,
		&game.ReturnMultiplierBPS, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return activity.Game{}, ErrNotFound
		}
		return activity.Game{}, err
	}
	game.Enabled = enabled == 1
	var err error
	if game.CreatedAt, err = parseStamp(created); err != nil {
		return activity.Game{}, err
	}
	game.UpdatedAt, err = parseStamp(updated)
	return game, err
}

const activityBetSelect = `SELECT id,user_id,game_id,stake_minor,won,payout_minor,balance_after_minor,configuration_snapshot,idempotency_key,created_at FROM activity_bets`

func (s *Store) activityBetByID(ctx context.Context, betID string) (activity.BetResult, error) {
	return scanActivityBet(s.db.QueryRowContext(ctx, activityBetSelect+` WHERE id=?`, betID))
}

func activityBetByKeyTx(ctx context.Context, tx *sql.Tx, userID, key string) (activity.BetResult, error) {
	return scanActivityBet(tx.QueryRowContext(ctx, activityBetSelect+` WHERE user_id=? AND idempotency_key=?`, userID, key))
}

func scanActivityBet(row rowScanner) (activity.BetResult, error) {
	var result activity.BetResult
	var won int
	var created string
	if err := row.Scan(&result.ID, &result.UserID, &result.GameID, &result.StakeMinor, &won, &result.PayoutMinor, &result.BalanceAfterMinor,
		&result.ConfigurationSnapshot, &result.IdempotencyKey, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return activity.BetResult{}, ErrNotFound
		}
		return activity.BetResult{}, err
	}
	result.Won = won == 1
	parsed, err := parseStamp(created)
	result.CreatedAt = parsed
	return result, err
}

const dailyCheckInSelect = `SELECT id,user_id,local_date,timezone,reward_minor,balance_after_minor,created_at FROM activity_daily_checkins`

func dailyCheckInByDateTx(ctx context.Context, tx *sql.Tx, userID, localDate string) (activity.DailyCheckIn, error) {
	return scanDailyCheckIn(tx.QueryRowContext(ctx, dailyCheckInSelect+` WHERE user_id=? AND local_date=?`, userID, localDate))
}

func (s *Store) dailyCheckInByID(ctx context.Context, checkInID string) (activity.DailyCheckIn, error) {
	return scanDailyCheckIn(s.db.QueryRowContext(ctx, dailyCheckInSelect+` WHERE id=?`, checkInID))
}

func scanDailyCheckIn(row rowScanner) (activity.DailyCheckIn, error) {
	var result activity.DailyCheckIn
	var created string
	if err := row.Scan(&result.ID, &result.UserID, &result.LocalDate, &result.Timezone, &result.RewardMinor, &result.BalanceAfterMinor, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return activity.DailyCheckIn{}, ErrNotFound
		}
		return activity.DailyCheckIn{}, err
	}
	parsed, err := parseStamp(created)
	result.CreatedAt = parsed
	return result, err
}

const luckyDrawSelect = `SELECT id,name,description,kind,status,fee_minor,expected_participation,threshold,
 keyword,command,group_chat_id,announcement_message_id,revision,created_at,updated_at FROM activity_lucky_draws`

func luckyDrawByID(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, drawID string, enabledOnly bool) (activity.LuckyDraw, error) {
	query := luckyDrawSelect + ` WHERE id=?`
	if enabledOnly {
		query += ` AND kind='instant' AND status='open'`
	}
	return scanLuckyDraw(queryer.QueryRowContext(ctx, query, drawID))
}

func scanLuckyDraw(row rowScanner) (activity.LuckyDraw, error) {
	var draw activity.LuckyDraw
	var expected, threshold sql.NullInt64
	var groupID, announcementID sql.NullInt64
	var created, updated string
	if err := row.Scan(&draw.ID, &draw.Name, &draw.Description, &draw.Kind, &draw.Status, &draw.FeeMinor,
		&expected, &threshold, &draw.Keyword, &draw.Command, &groupID, &announcementID, &draw.Revision, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return activity.LuckyDraw{}, ErrNotFound
		}
		return activity.LuckyDraw{}, err
	}
	draw.Enabled = draw.Status == "open"
	if expected.Valid {
		draw.ExpectedParticipation = int(expected.Int64)
	}
	if threshold.Valid {
		draw.Threshold = int(threshold.Int64)
	}
	if groupID.Valid {
		draw.GroupChatID = groupID.Int64
	}
	if announcementID.Valid {
		draw.AnnouncementMessageID = announcementID.Int64
	}
	var err error
	if draw.CreatedAt, err = parseStamp(created); err != nil {
		return activity.LuckyDraw{}, err
	}
	draw.UpdatedAt, err = parseStamp(updated)
	return draw, err
}

func luckyPrizes(ctx context.Context, queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, drawID string, availableOnly bool) ([]activity.PrizeInput, error) {
	query := `SELECT id,name,probability_bps,stock,reward_payload FROM activity_lucky_prizes WHERE draw_id=?`
	query += ` ORDER BY position`
	rows, err := queryer.QueryContext(ctx, query, drawID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]activity.PrizeInput, 0)
	for rows.Next() {
		var prize activity.PrizeInput
		var stock, probability sql.NullInt64
		var payload string
		if err := rows.Scan(&prize.ID, &prize.Name, &probability, &stock, &payload); err != nil {
			return nil, err
		}
		if stock.Valid {
			prize.Stock = stock.Int64
		}
		if probability.Valid {
			prize.ProbabilityBPS = int(probability.Int64)
		}
		if err := json.Unmarshal([]byte(payload), &prize.Reward); err != nil {
			return nil, fmt.Errorf("decode lucky-draw reward: %w", err)
		}
		result = append(result, prize)
	}
	return result, rows.Err()
}

const drawResultSelect = `SELECT id,user_id,draw_id,prize_id,prize_name,fee_minor,reward_payload,balance_after_minor,configuration_snapshot,idempotency_key,created_at FROM activity_draw_results`

func drawResultByKeyTx(ctx context.Context, tx *sql.Tx, userID, key string) (activity.DrawResult, error) {
	return scanDrawResult(tx.QueryRowContext(ctx, drawResultSelect+` WHERE user_id=? AND idempotency_key=?`, userID, key))
}

func (s *Store) drawResultByID(ctx context.Context, resultID string) (activity.DrawResult, error) {
	return scanDrawResult(s.db.QueryRowContext(ctx, drawResultSelect+` WHERE id=?`, resultID))
}

func scanDrawResult(row rowScanner) (activity.DrawResult, error) {
	var result activity.DrawResult
	var payload, created string
	if err := row.Scan(&result.ID, &result.UserID, &result.DrawID, &result.PrizeID, &result.PrizeName, &result.FeeMinor, &payload,
		&result.BalanceAfterMinor, &result.ConfigurationSnapshot, &result.IdempotencyKey, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return activity.DrawResult{}, ErrNotFound
		}
		return activity.DrawResult{}, err
	}
	if err := json.Unmarshal([]byte(payload), &result.Reward); err != nil {
		return activity.DrawResult{}, err
	}
	parsed, err := parseStamp(created)
	result.CreatedAt = parsed
	return result, err
}
