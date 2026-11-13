package admin

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type activitySettingsFixture struct {
	*adminSettingsRepository
	config activity.CheckInConfig
	actor  string
	calls  int
}

func (r *activitySettingsFixture) SaveActivitySettings(_ context.Context, actor string, config activity.CheckInConfig, _ time.Time) (activity.CheckInConfig, error) {
	r.calls++
	if r.putErr != nil {
		return activity.CheckInConfig{}, r.putErr
	}
	r.actor, r.config = actor, config
	return config, nil
}

func TestActivityConfigUsesOneSnapshotAndPreservesStorageErrors(t *testing.T) {
	t.Parallel()
	repository := newAdminSettingsRepository()
	repository.getErr = errors.New("per-key reads must not be used")
	service := NewSettingsService(repository, testVault(t))
	ctx := context.Background()
	config, err := service.ActivityConfig(ctx)
	if err != nil || config != (activity.CheckInConfig{Timezone: activity.DefaultTimezone}) {
		t.Fatal(config, err)
	}
	for key, value := range map[string]string{
		database.ActivityTimezoneSetting: "UTC", database.ActivityRewardMinSetting: "1.01", database.ActivityRewardMaxSetting: "2.99",
		database.ActivityMessageThresholdSetting: "123", database.ActivityMessageRewardSetting: "0.25",
	} {
		repository.settings[key] = database.Setting{Key: key, Value: value}
	}
	config, err = service.ActivityConfig(ctx)
	if err != nil || config != (activity.CheckInConfig{Timezone: "UTC", RewardMinMinor: 101, RewardMaxMinor: 299, GroupMessageThreshold: 123, GroupMessageRewardMinor: 25}) {
		t.Fatal(config, err)
	}
	repository.listErr = errors.New("temporary storage failure")
	if _, err := service.ActivityConfig(ctx); !errors.Is(err, repository.listErr) || errors.Is(err, ErrActivityConfiguration) {
		t.Fatal("storage error changed to invalid configuration", err)
	}
	repository.listErr = nil
	repository.settings[database.ActivityRewardMaxSetting] = database.Setting{Key: database.ActivityRewardMaxSetting, Value: "0.01"}
	if _, err := service.ActivityConfig(ctx); !errors.Is(err, ErrActivityConfiguration) {
		t.Fatal("invalid stored range accepted", err)
	}
}

func TestSaveActivityConfigUsesAtomicAdapterAndValidatesAllFields(t *testing.T) {
	t.Parallel()
	repository := &activitySettingsFixture{adminSettingsRepository: newAdminSettingsRepository()}
	service := NewSettingsService(repository, testVault(t))
	ctx := context.Background()
	base := activity.CheckInConfig{Timezone: " UTC ", RewardMinMinor: 101, RewardMaxMinor: 299, GroupMessageThreshold: 5, GroupMessageRewardMinor: 50}
	got, err := service.SaveActivityConfig(ctx, "admin", base)
	base.Timezone = "UTC"
	if err != nil || got != base || repository.config != base || repository.actor != "admin" || repository.calls != 1 || repository.putCalls != 0 {
		t.Fatal("save did not use one atomic adapter", got, err)
	}
	for _, modify := range []func(*activity.CheckInConfig){
		func(c *activity.CheckInConfig) { c.Timezone = "invalid/timezone" },
		func(c *activity.CheckInConfig) { c.RewardMinMinor = -1 },
		func(c *activity.CheckInConfig) { c.RewardMaxMinor = c.RewardMinMinor - 1 },
		func(c *activity.CheckInConfig) { c.GroupMessageThreshold = -1 },
		func(c *activity.CheckInConfig) { c.GroupMessageThreshold = math.MaxInt32 + 1 },
		func(c *activity.CheckInConfig) { c.GroupMessageRewardMinor = -1 },
	} {
		invalid := base
		modify(&invalid)
		if _, err := service.SaveActivityConfig(ctx, "admin", invalid); !errors.Is(err, activity.ErrInvalidInput) {
			t.Fatal("invalid bundle accepted", invalid, err)
		}
	}
	if repository.calls != 1 {
		t.Fatal("invalid input reached persistence")
	}
	repository.putErr = errors.New("database unavailable")
	if _, err := service.SaveActivityConfig(ctx, "admin", base); !errors.Is(err, repository.putErr) {
		t.Fatal("write error discarded", err)
	}
	if _, err := NewSettingsService(newAdminSettingsRepository(), testVault(t)).SaveActivityConfig(ctx, "admin", base); err == nil {
		t.Fatal("missing atomic adapter fell back to per-key writes")
	}
}
