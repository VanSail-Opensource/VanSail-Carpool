package admin

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func init() {
	settingDefinitions[xnode.GlobalSettingKey] = SettingDefinition{Secret: true, Validate: func(raw string) error { _, err := xnode.DecodeGlobal(raw); return err }}
}

// PutXNodeGlobal is used only after the fleet service performs queued link validation.
func (s *SettingsService) PutXNodeGlobal(ctx context.Context, actor, raw string) error {
	return s.put(ctx, actor, xnode.GlobalSettingKey, raw)
}
