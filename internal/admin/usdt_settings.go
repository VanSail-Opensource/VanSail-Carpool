package admin

import "github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"

func init() {
	settingDefinitions[usdt.ConfigKey] = SettingDefinition{Secret: true, Validate: func(raw string) error { _, err := usdt.DecodeConfig(raw); return err }}
}
