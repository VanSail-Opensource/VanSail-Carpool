package database

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func TestActivitySettingsAuditFailurePreservesCompleteBundle(t *testing.T) {
	t.Parallel()
	ctx, store, now := context.Background(), newTestStore(t), time.Now().UTC()
	actor := activityAdminActor(t, store)
	initial := activity.CheckInConfig{Timezone: "UTC", RewardMinMinor: 100, RewardMaxMinor: 200, GroupMessageThreshold: 10, GroupMessageRewardMinor: 50}
	if _, err := store.SaveActivitySettings(ctx, actor, initial, now); err != nil {
		t.Fatal(err)
	}
	before, err := store.ListSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	next := activity.CheckInConfig{Timezone: " Asia/Shanghai ", RewardMinMinor: 1000, RewardMaxMinor: 2000, GroupMessageThreshold: 20, GroupMessageRewardMinor: 70}
	rejectActivityAudit(t, store)
	if _, err := store.SaveActivitySettings(ctx, actor, next, now.Add(time.Second)); err == nil {
		t.Fatal("bundle committed without audit")
	}
	after, err := store.ListSettings(ctx)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("failed audit partially published settings", after, err)
	}
	allowActivityAudit(t, store)
	committed, err := store.SaveActivitySettings(ctx, actor, next, now.Add(time.Second))
	next.Timezone = "Asia/Shanghai"
	if err != nil || committed != next {
		t.Fatal("retry did not return exact committed bundle", committed, err)
	}
	var audits int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1 AND action='activity.settings.update' AND target_id='activity'`, actor).Scan(&audits); err != nil || audits != 2 {
		t.Fatal(audits, err)
	}
	bad := next
	bad.RewardMaxMinor = bad.RewardMinMinor - 1
	if _, err := store.SaveActivitySettings(ctx, actor, bad, now); !errors.Is(err, activity.ErrInvalidInput) {
		t.Fatal("invalid range was published", err)
	}
	var maximum string
	if err := store.DB().QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1`, ActivityRewardMaxSetting).Scan(&maximum); err != nil || maximum != "20.00" {
		t.Fatal(maximum, err)
	}
}

func TestActivitySettingsSnapshotsNeverMixConcurrentBundles(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, now := newTestStore(t), time.Now().UTC()
	actor := activityAdminActor(t, store)
	configs := []activity.CheckInConfig{
		{Timezone: "UTC", RewardMinMinor: 100, RewardMaxMinor: 200, GroupMessageThreshold: 10, GroupMessageRewardMinor: 50},
		{Timezone: "Asia/Shanghai", RewardMinMinor: 1000, RewardMaxMinor: 2000, GroupMessageThreshold: 20, GroupMessageRewardMinor: 70},
	}
	want := []map[string]string{
		{ActivityTimezoneSetting: "UTC", ActivityRewardMinSetting: "1.00", ActivityRewardMaxSetting: "2.00", ActivityMessageThresholdSetting: "10", ActivityMessageRewardSetting: "0.50"},
		{ActivityTimezoneSetting: "Asia/Shanghai", ActivityRewardMinSetting: "10.00", ActivityRewardMaxSetting: "20.00", ActivityMessageThresholdSetting: "20", ActivityMessageRewardSetting: "0.70"},
	}
	if _, err := store.SaveActivitySettings(ctx, actor, configs[0], now); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 3)
	for _, config := range configs {
		go func(config activity.CheckInConfig) {
			for i := 0; i < 10; i++ {
				if _, err := store.SaveActivitySettings(ctx, actor, config, now.Add(time.Duration(i)*time.Second)); err != nil {
					done <- err
					return
				}
			}
			done <- nil
		}(config)
	}
	go func() {
		for i := 0; i < 100; i++ {
			rows, err := store.ListSettings(ctx)
			if err != nil {
				done <- err
				return
			}
			values := map[string]string{}
			for _, row := range rows {
				if _, ok := want[0][row.Key]; ok {
					values[row.Key] = row.Value
				}
			}
			if !reflect.DeepEqual(values, want[0]) && !reflect.DeepEqual(values, want[1]) {
				done <- errors.New("reader observed values from different committed bundles")
				return
			}
		}
		done <- nil
	}()
	for i := 0; i < 3; i++ {
		if err := <-done; err != nil {
			t.Error(err)
		}
	}
}
