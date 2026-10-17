package admin

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func init() {
	settingDefinitions[xnode.GlobalSettingKey] = SettingDefinition{Secret: true, Validate: func(raw string) error { _, err := xnode.DecodeGlobal(raw); return err }}
	settingDefinitions[xnode.SubscriptionTemplatesKey] = SettingDefinition{Secret: true, Validate: func(raw string) error { _, err := xnode.DecodeSubscriptionTemplates(raw); return err }}
	settingDefinitions[xnode.RoutingTemplatesKey] = SettingDefinition{Secret: true, Validate: func(raw string) error { _, err := xnode.DecodeRoutingTemplates(raw); return err }}
}

func (s *SettingsService) PutXNodeRoutingTemplates(ctx context.Context, actor, raw string) error {
	return s.put(ctx, actor, xnode.RoutingTemplatesKey, raw)
}

// PutXNodeSubscriptionTemplates accepts only the validated fleet service path.
func (s *SettingsService) PutXNodeSubscriptionTemplates(ctx context.Context, actor, raw string) error {
	return s.put(ctx, actor, xnode.SubscriptionTemplatesKey, raw)
}

// PutXNodeGlobal is used only after the fleet service performs queued link validation.
func (s *SettingsService) PutXNodeGlobal(ctx context.Context, actor, raw string) error {
	return s.put(ctx, actor, xnode.GlobalSettingKey, raw)
}
