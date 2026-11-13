package app

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type embyPriceSettings struct {
	admin.SettingsRepository
	value string
	err   error
}

func (r embyPriceSettings) GetSetting(_ context.Context, key string) (database.Setting, error) {
	return database.Setting{Key: key, Value: r.value}, r.err
}

func TestEmbyPriceDistinguishesInvalidConfigurationFromStorageFailure(t *testing.T) {
	storageErr := errors.New("private storage failure")
	for _, item := range []struct {
		name  string
		value string
		err   error
		want  error
		minor int64
	}{
		{name: "free setup", value: "0"},
		{name: "valid setup", value: "12.50", minor: 1250},
		{name: "empty", want: emby.ErrInvalidSetup},
		{name: "negative", value: "-1", want: emby.ErrInvalidSetup},
		{name: "malformed", value: "private-invalid-price", want: emby.ErrInvalidSetup},
		{name: "overflow", value: "9223372036854775808", want: emby.ErrInvalidSetup},
		{name: "absent", err: database.ErrNotFound, want: database.ErrNotFound},
		{name: "storage outage", err: storageErr, want: storageErr},
	} {
		t.Run(item.name, func(t *testing.T) {
			settings := admin.NewSettingsService(embyPriceSettings{value: item.value, err: item.err}, nil)
			minor, err := embySetupPrice(settings).EmbySetupPriceTXBMinor(context.Background())
			if !errors.Is(err, item.want) || minor != item.minor {
				t.Fatalf("price=%d, error=%v; want price=%d, error=%v", minor, err, item.minor, item.want)
			}
		})
	}
}
