package database

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

const (
	ActivityTimezoneSetting         = "activity.timezone"
	ActivityRewardMinSetting        = "activity.daily_reward_min_txb"
	ActivityRewardMaxSetting        = "activity.daily_reward_max_txb"
	ActivityMessageThresholdSetting = "activity.group_message_threshold"
	ActivityMessageRewardSetting    = "activity.group_message_reward_txb"
)

// SaveActivitySettings writes the coupled daily range and message reward policy
// as one snapshot. The existing five rows remain the only configuration store.
func (s *Store) retryAttempt_SaveActivitySettings(ctx context.Context, actorID string, config activity.CheckInConfig, now time.Time) (activity.CheckInConfig, error) {
	config.Timezone = strings.TrimSpace(config.Timezone)
	if err := config.Validate(); err != nil {
		return activity.CheckInConfig{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return activity.CheckInConfig{}, err
	}
	defer tx.Rollback()
	if err = catalogAdminActorTx(ctx, tx, actorID); err != nil {
		return activity.CheckInConfig{}, err
	}
	major := func(minor int64) string { return fmt.Sprintf("%d.%02d", minor/100, minor%100) }
	// Every writer takes the same row order, including first-time initialization.
	for _, setting := range []struct{ key, value string }{
		{ActivityRewardMaxSetting, major(config.RewardMaxMinor)},
		{ActivityRewardMinSetting, major(config.RewardMinMinor)},
		{ActivityMessageRewardSetting, major(config.GroupMessageRewardMinor)},
		{ActivityMessageThresholdSetting, strconv.Itoa(config.GroupMessageThreshold)},
		{ActivityTimezoneSetting, config.Timezone},
	} {
		if _, err = tx.ExecContext(ctx, `INSERT INTO settings(key,value,encrypted,updated_at,updated_by)
			VALUES($1,$2,0,$3,$4) ON CONFLICT(key) DO UPDATE SET value=excluded.value,
			encrypted=0,updated_at=excluded.updated_at,updated_by=excluded.updated_by`,
			setting.key, setting.value, stamp(now.UTC()), actorID); err != nil {
			return activity.CheckInConfig{}, err
		}
	}
	if err = catalogAdminAuditTx(ctx, tx, actorID, "activity.settings.update", "setting", "activity", config, now.UTC()); err != nil {
		return activity.CheckInConfig{}, err
	}
	if err = tx.Commit(); err != nil {
		return activity.CheckInConfig{}, err
	}
	return config, nil
}
