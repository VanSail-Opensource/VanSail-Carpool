package admin

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
)

// SetConnectivityValidation validates the same atomic config for all settings
// write paths and invalidates process-owned checks only after persistence.
func (s *SettingsService) SetConnectivityValidation(validate func(context.Context, connectivity.Config) error, changed func()) {
	s.connectivityValidator = validate
	s.connectivityChanged = changed
}

func (s *SettingsService) validateConnectivitySettings(ctx context.Context, key, value string) error {
	if key != connectivity.SettingKey {
		return nil
	}
	config, err := connectivity.DecodeConfig(value)
	if err != nil {
		return err
	}
	previous, readErr := s.Optional(ctx, connectivity.SettingKey)
	if readErr != nil {
		return readErr
	}
	stored, decodeErr := connectivity.DecodeConfig(previous)
	if decodeErr == nil && stored == config {
		return nil
	}
	// Disabling an unchanged account must remain possible during an upstream
	// outage or after the account expires. New selections still require validation.
	if !config.ScheduledEnabled && config.ProxyUserID > 0 {
		if decodeErr == nil && stored.ProxyUserID == config.ProxyUserID {
			return nil
		}
	}
	if s.connectivityValidator != nil {
		return s.connectivityValidator(ctx, config)
	}
	if config.ProxyUserID > 0 {
		return &connectivity.CodeError{Code: "CONNECTIVITY_VALIDATION_UNAVAILABLE"}
	}
	return nil
}

func (s *SettingsService) connectivitySettingChanged(ctx context.Context, key, value string) (bool, error) {
	if key != connectivity.SettingKey {
		return false, nil
	}
	previous, err := s.Optional(ctx, key)
	if err != nil {
		return false, err
	}
	if key == connectivity.SettingKey {
		before, beforeErr := connectivity.DecodeConfig(previous)
		after, afterErr := connectivity.DecodeConfig(value)
		if beforeErr == nil && afterErr == nil {
			return before != after, nil
		}
	}
	return previous != value, nil
}
