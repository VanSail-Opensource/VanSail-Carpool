package activity

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const DefaultTimezone = "Asia/Shanghai"

// Validate checks the complete settings bundle before any value is published.
func (config CheckInConfig) Validate() error {
	if strings.TrimSpace(config.Timezone) == "" || config.RewardMinMinor < 0 || config.RewardMaxMinor < config.RewardMinMinor ||
		config.GroupMessageThreshold < 0 || config.GroupMessageThreshold > math.MaxInt32 || config.GroupMessageRewardMinor < 0 {
		return fmt.Errorf("%w: invalid activity reward configuration", ErrInvalidInput)
	}
	if _, err := time.LoadLocation(config.Timezone); err != nil {
		return fmt.Errorf("%w: unknown activity timezone", ErrInvalidInput)
	}
	return nil
}
