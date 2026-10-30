package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
)

const (
	activityTimezoneSetting      = "activity.timezone"
	activityRewardMinSetting     = "activity.daily_reward_min_txb"
	activityRewardMaxSetting     = "activity.daily_reward_max_txb"
	groupMessageThresholdSetting = "activity.group_message_threshold"
	groupMessageRewardSetting    = "activity.group_message_reward_txb"
	defaultActivityTimezone      = "Asia/Shanghai"
	maxMultipartOverhead         = int64(256 << 10)
)

var errActivityConfiguration = errors.New("invalid activity configuration")

type nullableRequestField[T any] struct {
	Set   bool
	Value *T
}

func (field *nullableRequestField[T]) UnmarshalJSON(data []byte) error {
	field.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		field.Value = nil
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	field.Value = &value
	return nil
}
