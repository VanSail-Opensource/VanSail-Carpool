package admin

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
)

func TestConnectivitySettingsValidationAndDisableDuringOutage(t *testing.T) {
	repository := newAdminSettingsRepository()
	service := NewSettingsService(repository, testVault(t))
	validations, invalidations := 0, 0
	var upstreamErr error
	service.SetConnectivityValidation(func(context.Context, connectivity.Config) error {
		validations++
		return upstreamErr
	}, func() { invalidations++ })
	ctx := context.Background()
	cfg := connectivity.DefaultConfig()
	cfg.RemnawaveUserID, cfg.ScheduledEnabled = 7, true
	write := func() error {
		value, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		return service.Put(ctx, "admin", connectivity.SettingKey, string(value))
	}
	if err := write(); err != nil {
		t.Fatal(err)
	}
	if validations != 1 || invalidations != 1 {
		t.Fatal("selection was not validated and invalidated")
	}
	upstreamErr = errors.New("upstream unavailable")
	if err := write(); err != nil {
		t.Fatalf("identical settings should remain writable: %v", err)
	}
	if validations != 1 || invalidations != 1 {
		t.Fatal("identical settings disturbed an existing run")
	}
	cfg.ScheduledEnabled = false
	if err := write(); err != nil {
		t.Fatalf("disable during outage: %v", err)
	}
	if validations != 1 || invalidations != 2 {
		t.Fatal("disable should not require upstream access")
	}
	cfg.RemnawaveUserID = 9
	if err := write(); err == nil {
		t.Fatal("new selection bypassed validation")
	}
	if invalidations != 2 {
		t.Fatal("failed save invalidated running work")
	}
	if err := service.Put(ctx, "admin", connectivity.SettingKey, `{"scheduledEnabled":true,"remnawaveUserId":0}`); err == nil {
		t.Fatal("generic settings path accepted enabled unconfigured checker")
	}
}
