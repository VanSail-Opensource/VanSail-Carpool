package billing

import (
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestPaymentChannelCacheBindsEveryReadAndMutationToProfileRevision(t *testing.T) {
	cache := NewPaymentChannelCache()
	first := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	second := first.Add(time.Nanosecond)
	oldChannels := []model.PaymentChannel{{Rail: "usdt.trc20", Currency: "USDT", Network: "trc20"}}
	newChannels := []model.PaymentChannel{{Rail: "usdt.bep20", Currency: "USDT", Network: "bep20"}}
	cache.Replace("profile", first, oldChannels)
	if got := cache.PaymentProfileRails("profile", second); len(got) != 0 {
		t.Fatal("new profile inherited stale rails before its discovery completed", got)
	}
	cache.Replace("profile", second, newChannels)
	cache.Replace("profile", first, oldChannels)
	cache.Remove("profile", first)
	if got := cache.PaymentProfileRails("profile", second); len(got) != 1 || got[0] != "usdt.bep20" {
		t.Fatal("late discovery or cleanup replaced current rails", got)
	}
	if got := cache.PaymentProfileRails("profile", first); len(got) != 0 {
		t.Fatal("old profile read another revision's rails", got)
	}
	newChannels[0].Rail = "mutated"
	copy := cache.PaymentProfileChannels("profile", second)
	copy[0].Rail = "also-mutated"
	if got := cache.PaymentProfileRails("profile", second); len(got) != 1 || got[0] != "usdt.bep20" {
		t.Fatal("callers mutated the retained discovery", got)
	}
	cache.Remove("profile", second)
	if !cache.Revision("profile").IsZero() || len(cache.PaymentProfileRails("profile", second)) != 0 {
		t.Fatal("current revision cleanup retained rails")
	}
	cache.Replace("profile", time.Time{}, oldChannels)
	if len(cache.PaymentProfileRails("profile", time.Time{})) != 0 {
		t.Fatal("missing durable revision authorized rails")
	}
}
