package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/billing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestPaymentDiscoveryCannotReplaceOrDisableASavedProfile(t *testing.T) {
	ctx := context.Background()
	adapter, base := paymentProfileAdapterForTest(t, "bepusdt")
	store := &revisionPaymentProfileStore{paymentProfileTestStore: base}
	adapter.settings.SetPaymentProfileRepository(store)
	cache := billing.NewPaymentChannelCache()
	adapter.settings.SetPaymentProfileChannels(cache)
	first := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	base.profiles[0].Enabled, base.profiles[0].UpdatedAt = true, first
	probed, err := adapter.settings.PaymentProfileByID(ctx, "first", "")
	if err != nil {
		t.Fatal(err)
	}
	manager := &PaymentProfileManager{settings: adapter.settings, cache: cache, admins: []int64{1}}
	base.profiles[0].UpdatedAt, base.profiles[0].Endpoint = first.Add(time.Nanosecond), "https://replacement.example"
	current := base.profiles[0].PaymentProfile
	cache.Replace(current.ID, current.UpdatedAt, []model.PaymentChannel{{Rail: "usdt.bep20"}})
	if result, err := manager.publishChannels(ctx, probed, []model.PaymentChannel{{Rail: "usdt.trc20"}}); err == nil || !result.UpdatedAt.Equal(current.UpdatedAt) {
		t.Fatalf("old successful discovery replaced saved profile: result=%+v err=%v", result, err)
	}
	cause := errors.New("old account discovery failed")
	result, err := manager.disableFailed(ctx, probed.PaymentProfile, cause)
	if !errors.Is(err, cause) || !result.Enabled || !result.UpdatedAt.Equal(current.UpdatedAt) || store.disables != 0 {
		t.Fatalf("old failed discovery disabled saved profile: result=%+v disables=%d err=%v", result, store.disables, err)
	}
	// No Telegram client is installed: a stale failure must not report that the
	// replacement profile was disabled. Its current discovery remains available.
	if got := cache.PaymentProfileRails(current.ID, current.UpdatedAt); len(got) != 1 || got[0] != "usdt.bep20" {
		t.Fatal("stale discovery erased new rails", got)
	}
	newRuntime, err := adapter.settings.PaymentProfileByID(ctx, current.ID, "usdt.bep20")
	if err != nil || newRuntime.Endpoint != current.Endpoint {
		t.Fatal("new profile could not use its own discovery", err)
	}
	if result, err := manager.publishChannels(ctx, newRuntime, []model.PaymentChannel{{Rail: "usdt.erc20"}}); err != nil || len(result.EnabledChannels) != 1 || result.EnabledChannels[0] != "usdt.erc20" {
		t.Fatal("current discovery did not publish", result, err)
	}
}

func TestPaymentProfileDeletionCannotEraseReplacementDiscovery(t *testing.T) {
	adapter, base := paymentProfileAdapterForTest(t, "bepusdt")
	cache := billing.NewPaymentChannelCache()
	first := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	second := first.Add(time.Nanosecond)
	cache.Replace("first", first, []model.PaymentChannel{{Rail: "usdt.trc20"}})
	store := &revisionPaymentProfileStore{paymentProfileTestStore: base, deleted: func() {
		// A new configuration with the same ID was saved and discovered after
		// the delete transaction committed, before old cache cleanup completed.
		cache.Replace("first", second, []model.PaymentChannel{{Rail: "usdt.bep20"}})
	}}
	adapter.settings.SetPaymentProfileRepository(store)
	manager := &PaymentProfileManager{settings: adapter.settings, cache: cache}
	if err := manager.DeletePaymentProfile(context.Background(), "first", "admin"); err != nil {
		t.Fatal(err)
	}
	if got := cache.PaymentProfileRails("first", second); len(got) != 1 || got[0] != "usdt.bep20" {
		t.Fatal("old deletion erased replacement rails", got)
	}
}

type revisionPaymentProfileStore struct {
	*paymentProfileTestStore
	disables int
	deleted  func()
}

func (s *revisionPaymentProfileStore) DisablePaymentProfileIfUnchanged(ctx context.Context, id string, expected time.Time) (model.PaymentProfile, bool, error) {
	record, err := s.PaymentProfileRecordByID(ctx, id, "")
	if err != nil || !record.Enabled || !record.UpdatedAt.Equal(expected) {
		return record.PaymentProfile, false, err
	}
	s.disables++
	record.Enabled = false
	return record.PaymentProfile, true, nil
}

func (s *revisionPaymentProfileStore) DeletePaymentProfile(context.Context, string, string) error {
	s.deleted()
	return nil
}
