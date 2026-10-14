package httpapi

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"strconv"
	"strings"
	"time"
)

func (s *Server) activityConfig(ctx context.Context) (activity.CheckInConfig, error) {
	timezone, err := s.deps.Settings.Optional(ctx, activityTimezoneSetting)
	if err != nil {
		return activity.CheckInConfig{}, err
	}
	if strings.TrimSpace(timezone) == "" {
		timezone = defaultActivityTimezone
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return activity.CheckInConfig{}, errActivityConfiguration
	}
	rewardMinValue, err := s.deps.Settings.Optional(ctx, activityRewardMinSetting)
	if err != nil {
		return activity.CheckInConfig{}, err
	}
	if strings.TrimSpace(rewardMinValue) == "" {
		rewardMinValue = "0"
	}
	rewardMinMinor, err := billing.ParseTXBMajor(rewardMinValue)
	if err != nil || rewardMinMinor < 0 {
		return activity.CheckInConfig{}, errActivityConfiguration
	}
	rewardMaxValue, err := s.deps.Settings.Optional(ctx, activityRewardMaxSetting)
	if err != nil {
		return activity.CheckInConfig{}, err
	}
	if strings.TrimSpace(rewardMaxValue) == "" {
		rewardMaxValue = "0"
	}
	rewardMaxMinor, err := billing.ParseTXBMajor(rewardMaxValue)
	if err != nil || rewardMaxMinor < rewardMinMinor {
		return activity.CheckInConfig{}, errActivityConfiguration
	}
	thresholdValue, err := s.deps.Settings.Optional(ctx, groupMessageThresholdSetting)
	if err != nil {
		return activity.CheckInConfig{}, err
	}
	if strings.TrimSpace(thresholdValue) == "" {
		thresholdValue = "0"
	}
	threshold, err := strconv.ParseInt(thresholdValue, 10, 32)
	if err != nil || threshold < 0 {
		return activity.CheckInConfig{}, errActivityConfiguration
	}
	groupRewardValue, err := s.deps.Settings.Optional(ctx, groupMessageRewardSetting)
	if err != nil {
		return activity.CheckInConfig{}, err
	}
	if strings.TrimSpace(groupRewardValue) == "" {
		groupRewardValue = "0"
	}
	groupRewardMinor, err := billing.ParseTXBMajor(groupRewardValue)
	if err != nil || groupRewardMinor < 0 {
		return activity.CheckInConfig{}, errActivityConfiguration
	}
	return activity.CheckInConfig{Timezone: timezone, RewardMinMinor: rewardMinMinor, RewardMaxMinor: rewardMaxMinor, GroupMessageThreshold: int(threshold), GroupMessageRewardMinor: groupRewardMinor}, nil
}
