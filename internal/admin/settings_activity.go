package admin

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

var ErrActivityConfiguration = errors.New("invalid stored activity configuration")

type activitySettingsRepository interface {
	SaveActivitySettings(context.Context, string, activity.CheckInConfig, time.Time) (activity.CheckInConfig, error)
}

// ActivityConfig reads the complete bundle from one repository snapshot.
func (s *SettingsService) ActivityConfig(ctx context.Context) (activity.CheckInConfig, error) {
	settings, err := s.repository.ListSettings(ctx)
	if err != nil {
		return activity.CheckInConfig{}, err
	}
	values := map[string]string{
		database.ActivityTimezoneSetting:  activity.DefaultTimezone,
		database.ActivityRewardMinSetting: "0", database.ActivityRewardMaxSetting: "0",
		database.ActivityMessageThresholdSetting: "0", database.ActivityMessageRewardSetting: "0",
	}
	for _, setting := range settings {
		if _, ok := values[setting.Key]; !ok {
			continue
		}
		value := setting.Value
		if setting.Encrypted {
			if s.vault == nil {
				return activity.CheckInConfig{}, ErrActivityConfiguration
			}
			value, err = s.vault.Decrypt(setting.Key, value)
			if err != nil {
				return activity.CheckInConfig{}, ErrActivityConfiguration
			}
		}
		if value = strings.TrimSpace(value); value != "" {
			values[setting.Key] = value
		}
	}
	config := activity.CheckInConfig{Timezone: values[database.ActivityTimezoneSetting]}
	if config.RewardMinMinor, err = model.ParseTXBMajor(values[database.ActivityRewardMinSetting]); err != nil {
		return activity.CheckInConfig{}, ErrActivityConfiguration
	}
	if config.RewardMaxMinor, err = model.ParseTXBMajor(values[database.ActivityRewardMaxSetting]); err != nil {
		return activity.CheckInConfig{}, ErrActivityConfiguration
	}
	threshold, err := strconv.ParseInt(values[database.ActivityMessageThresholdSetting], 10, 32)
	if err != nil {
		return activity.CheckInConfig{}, ErrActivityConfiguration
	}
	config.GroupMessageThreshold = int(threshold)
	if config.GroupMessageRewardMinor, err = model.ParseTXBMajor(values[database.ActivityMessageRewardSetting]); err != nil {
		return activity.CheckInConfig{}, ErrActivityConfiguration
	}
	if err := config.Validate(); err != nil {
		return activity.CheckInConfig{}, ErrActivityConfiguration
	}
	return config, nil
}

// SaveActivityConfig publishes only complete, valid bundles through the atomic
// adapter. Generic per-key settings writes cannot bypass this boundary.
func (s *SettingsService) SaveActivityConfig(ctx context.Context, actorID string, config activity.CheckInConfig) (activity.CheckInConfig, error) {
	config.Timezone = strings.TrimSpace(config.Timezone)
	if err := config.Validate(); err != nil {
		return activity.CheckInConfig{}, err
	}
	repository, ok := s.repository.(activitySettingsRepository)
	if !ok {
		return activity.CheckInConfig{}, errors.New("activity settings repository is unavailable")
	}
	return repository.SaveActivitySettings(ctx, actorID, config, time.Now().UTC())
}
