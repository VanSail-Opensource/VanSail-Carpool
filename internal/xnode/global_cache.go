package xnode

import (
	"context"
	"maps"
	"slices"
	"sync"
)

// Only SaveGlobal writes this document after startup migrations. The published
// snapshot is private, and every reader receives independent mutable fields.
type globalSettingsCache struct {
	mu    sync.Mutex
	value *GlobalConfig
}

func (c *globalSettingsCache) read(ctx context.Context, settings Settings) (GlobalConfig, error) {
	if err := ctx.Err(); err != nil {
		return GlobalConfig{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.value == nil {
		raw, err := settings.Optional(ctx, GlobalSettingKey)
		if err != nil {
			return GlobalConfig{}, err
		}
		value, err := DecodeGlobal(raw)
		if err != nil {
			return GlobalConfig{}, err
		}
		c.value = &value
	}
	return cloneGlobal(*c.value), nil
}

func (c *globalSettingsCache) publish(value GlobalConfig) {
	value = cloneGlobal(value)
	c.mu.Lock()
	c.value = &value
	c.mu.Unlock()
}

func cloneGlobal(value GlobalConfig) GlobalConfig {
	if value.FirewallDetection != nil {
		copy := *value.FirewallDetection
		copy.Rules = maps.Clone(copy.Rules)
		value.FirewallDetection = &copy
	}
	if value.GeodataIntervalHours != nil {
		copy := *value.GeodataIntervalHours
		value.GeodataIntervalHours = &copy
	}
	value.SourceGeoIP.Selectors = slices.Clone(value.SourceGeoIP.Selectors)
	value.BlockedDomains = slices.Clone(value.BlockedDomains)
	value.Providers = maps.Clone(value.Providers)
	for name, provider := range value.Providers {
		provider.Credentials = maps.Clone(provider.Credentials)
		value.Providers[name] = provider
	}
	return value
}
