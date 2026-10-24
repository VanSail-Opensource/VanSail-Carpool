package database

import (
	"context"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"math"
	"strings"
	"time"
)

func (s *Store) retryAttempt_claimDailyActivity(ctx context.Context, userID, localDate, timezone string, minimumMinor, maximumMinor int64, boostCount int, rng activity.RandomSource, now time.Time) (activity.DailyCheckIn, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil || minimumMinor < 0 || maximumMinor < minimumMinor || maximumMinor-minimumMinor == math.MaxInt64 || strings.TrimSpace(userID) == "" {
		return activity.DailyCheckIn{}, activity.ErrInvalidInput
	}
	if _, err := time.Parse(time.DateOnly, localDate); err != nil || now.In(location).Format(time.DateOnly) != localDate {
		return activity.DailyCheckIn{}, activity.ErrInvalidInput
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	now = now.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return activity.DailyCheckIn{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if existing, loadErr := dailyCheckInByDateTx(ctx, tx, userID, localDate); loadErr == nil {
		existing.AlreadyClaimed = true
		return existing, nil
	} else if !errors.Is(loadErr, ErrNotFound) {
		return activity.DailyCheckIn{}, loadErr
	}
	rewardMinor := minimumMinor
	if maximumMinor > minimumMinor {
		if rng == nil {
			return activity.DailyCheckIn{}, activity.ErrInvalidInput
		}
		offset, randomErr := rng.Int63n(maximumMinor - minimumMinor + 1)
		if randomErr != nil {
			return activity.DailyCheckIn{}, randomErr
		}
		rewardMinor += offset
	}
	rewardMinor, err = activity.BoostRewardMinor(rewardMinor, boostCount)
	if err != nil {
		return activity.DailyCheckIn{}, err
	}
	checkInID, err := ids.New()
	if err != nil {
		return activity.DailyCheckIn{}, err
	}
	balance, err := changeBalanceTx(ctx, tx, userID, rewardMinor, now)
	if err != nil {
		return activity.DailyCheckIn{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_daily_checkins(id,user_id,local_date,timezone,reward_minor,balance_after_minor,created_at) VALUES(?,?,?,?,?,?,?)`,
		checkInID, userID, localDate, timezone, rewardMinor, balance, stamp(now))
	if err != nil {
		return activity.DailyCheckIn{}, fmt.Errorf("record daily activity: %w", err)
	}
	if _, err := insertLedgerTx(ctx, tx, userID, rewardMinor, balance, "activity_daily_checkin", checkInID, localDate, now); err != nil {
		return activity.DailyCheckIn{}, err
	}
	if err := tx.Commit(); err != nil {
		return activity.DailyCheckIn{}, err
	}
	return s.dailyCheckInByID(ctx, checkInID)
}
