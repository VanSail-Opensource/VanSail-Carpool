package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// ErrHourlyPolicyUnavailable rejects missing storage support or unreadable terms.
var ErrHourlyPolicyUnavailable = errors.New("hourly policy configuration is unavailable")

type hourlyPolicyRepository interface {
	ReadHourlyPolicyRecord(context.Context) (database.HourlyPolicyRecord, error)
	CompareAndSwapHourlyPolicy(context.Context, string, string, string) (database.HourlyPolicyRecord, error)
}

// HourlyPolicy reads and validates encrypted administrator terms outside Store.
// SalesEnabled gates new activation only; paid access still needs these terms.
func (s *SettingsService) HourlyPolicy(ctx context.Context) (model.HourlyPolicySnapshot, error) {
	repo, ok := s.repository.(hourlyPolicyRepository)
	if !ok {
		return model.HourlyPolicySnapshot{}, ErrHourlyPolicyUnavailable
	}
	record, err := repo.ReadHourlyPolicyRecord(ctx)
	if err != nil {
		return model.HourlyPolicySnapshot{}, err
	}
	if record.Ciphertext == "" && record.Version == "" {
		return model.HourlyPolicySnapshot{Policy: model.DefaultHourlyPolicy()}, nil
	}
	if s.vault == nil || record.Version == "" || record.Ciphertext == "" {
		return model.HourlyPolicySnapshot{}, ErrHourlyPolicyUnavailable
	}
	raw, err := s.vault.Decrypt(database.HourlyPolicySettingKey, record.Ciphertext)
	if err != nil {
		return model.HourlyPolicySnapshot{}, ErrHourlyPolicyUnavailable
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &fields) != nil {
		return model.HourlyPolicySnapshot{}, ErrHourlyPolicyUnavailable
	}
	for _, key := range []string{"salesEnabled", "dailyAllowanceBytes", "reducedRateBytesPerSecond", "extraGBPriceTXBMinor", "activationHours", "cooldownSeconds"} {
		if _, ok := fields[key]; !ok {
			return model.HourlyPolicySnapshot{}, ErrHourlyPolicyUnavailable
		}
	}
	var policy model.HourlyPolicy
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&policy); err != nil {
		return model.HourlyPolicySnapshot{}, ErrHourlyPolicyUnavailable
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF || model.ValidateHourlyPolicy(policy) != nil {
		return model.HourlyPolicySnapshot{}, ErrHourlyPolicyUnavailable
	}
	return model.HourlyPolicySnapshot{Version: record.Version, Policy: policy}, nil
}

// SaveHourlyPolicy encrypts a complete validated document before its SQL CAS.
func (s *SettingsService) SaveHourlyPolicy(ctx context.Context, actor, expectedVersion string, policy model.HourlyPolicy) (model.HourlyPolicySnapshot, error) {
	if err := model.ValidateHourlyPolicy(policy); err != nil {
		return model.HourlyPolicySnapshot{}, err
	}
	repo, ok := s.repository.(hourlyPolicyRepository)
	if !ok || s.vault == nil {
		return model.HourlyPolicySnapshot{}, ErrHourlyPolicyUnavailable
	}
	raw, err := json.Marshal(policy)
	if err != nil {
		return model.HourlyPolicySnapshot{}, err
	}
	ciphertext, err := s.vault.Encrypt(database.HourlyPolicySettingKey, string(raw))
	if err != nil {
		return model.HourlyPolicySnapshot{}, err
	}
	record, err := repo.CompareAndSwapHourlyPolicy(ctx, actor, expectedVersion, ciphertext)
	if err != nil {
		return model.HourlyPolicySnapshot{}, err
	}
	return model.HourlyPolicySnapshot{Version: record.Version, Policy: policy}, nil
}

// HourlyPolicy supplies the same validated snapshot to administrators and runtime.
func (s *Service) HourlyPolicy(ctx context.Context) (model.HourlyPolicySnapshot, error) {
	return s.settings.HourlyPolicy(ctx)
}

// SaveHourlyPolicy delegates audited CAS without a second non-atomic audit write.
func (s *Service) SaveHourlyPolicy(ctx context.Context, actor, expectedVersion string, policy model.HourlyPolicy) (model.HourlyPolicySnapshot, error) {
	return s.settings.SaveHourlyPolicy(ctx, actor, expectedVersion, policy)
}
