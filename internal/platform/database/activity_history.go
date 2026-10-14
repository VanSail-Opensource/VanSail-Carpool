package database

import (
	"context"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func (s *Store) ListActivityHistory(ctx context.Context, userID string, limit int) (activity.History, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	history := activity.History{
		Bets:     make([]activity.BetResult, 0),
		CheckIns: make([]activity.DailyCheckIn, 0),
		Draws:    make([]activity.DrawResult, 0),
	}
	betRows, err := s.db.QueryContext(ctx, activityBetSelect+` WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return activity.History{}, err
	}
	for betRows.Next() {
		result, scanErr := scanActivityBet(betRows)
		if scanErr != nil {
			_ = betRows.Close()
			return activity.History{}, scanErr
		}
		history.Bets = append(history.Bets, result)
	}
	if err := betRows.Err(); err != nil {
		_ = betRows.Close()
		return activity.History{}, err
	}
	_ = betRows.Close()
	checkInRows, err := s.db.QueryContext(ctx, dailyCheckInSelect+` WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return activity.History{}, err
	}
	for checkInRows.Next() {
		result, scanErr := scanDailyCheckIn(checkInRows)
		if scanErr != nil {
			_ = checkInRows.Close()
			return activity.History{}, scanErr
		}
		history.CheckIns = append(history.CheckIns, result)
	}
	if err := checkInRows.Err(); err != nil {
		_ = checkInRows.Close()
		return activity.History{}, err
	}
	_ = checkInRows.Close()
	drawRows, err := s.db.QueryContext(ctx, drawResultSelect+` WHERE user_id=? ORDER BY created_at DESC,id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return activity.History{}, err
	}
	defer func() { _ = drawRows.Close() }()
	for drawRows.Next() {
		result, scanErr := scanDrawResult(drawRows)
		if scanErr != nil {
			return activity.History{}, scanErr
		}
		history.Draws = append(history.Draws, result)
	}
	if err := drawRows.Err(); err != nil {
		return activity.History{}, err
	}
	return history, nil
}

// GroupMessageRewardStatus returns one user's current local-day group-message progress.

func (s *Store) GroupMessageRewardStatus(ctx context.Context, userID, localDate string, threshold int, rewardMinor int64) (activity.GroupMessageRewardStatus, error) {
	status, err := groupMessageRewardStatusQuery(ctx, s.db, userID, localDate, threshold, rewardMinor)
	if err != nil {
		return activity.GroupMessageRewardStatus{}, fmt.Errorf("load group-message reward status: %w", err)
	}
	return status, nil
}

// RecordGroupMessage idempotently counts one subscribed user's group message and credits the daily reward.
