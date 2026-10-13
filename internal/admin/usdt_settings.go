package admin

import "github.com/txyyddss/Remna-User-Panel/internal/usdt"

func init() {
	settingDefinitions[usdt.ConfigKey] = SettingDefinition{Secret: true, Validate: func(raw string) error { _, err := usdt.DecodeConfig(raw); return err }}
}
