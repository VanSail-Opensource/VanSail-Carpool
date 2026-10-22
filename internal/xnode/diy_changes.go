package xnode

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func sameDIYValue(a, b any) bool {
	x, err := json.Marshal(a)
	if err != nil {
		return false
	}
	y, err := json.Marshal(b)
	return err == nil && bytes.Equal(x, y)
}

func (s *Service) diySettingsForWrite(ctx context.Context, user string, input DIYWrite, proxies []subscription.Proxy) (DIYSettings, DIYSettings, error) {
	_, previous, err := s.readDIYSettings(ctx, user)
	if err != nil {
		return previous, previous, err
	}
	previous.Overrides = subscription.ActiveTuningFor(previous.Overrides, proxies)
	quota, err := s.whitelistQuota(ctx, user)
	if err != nil {
		return previous, previous, err
	}
	previous.Whitelist.IPs = previous.Whitelist.IPs[:min(len(previous.Whitelist.IPs), quota)]
	// Invalid saved references are automatic cleanup and do not count as a Save.
	previous.SplitProfiles = validSplitProfiles(previous.SplitProfiles, proxies)
	next := previous
	next.Overrides = subscription.ActiveTuningFor(input.Overrides, proxies)
	if input.Whitelist != nil {
		next.Whitelist, err = normalizeWhitelist(previous.Whitelist, *input.Whitelist, quota)
	}
	if err == nil && input.SplitProfiles != nil {
		err = validateSplitProfiles(*input.SplitProfiles, proxies)
		if err == nil {
			next.SplitProfiles = *input.SplitProfiles
		}
	}
	return previous, next, err
}
