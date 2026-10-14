package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type boostSettings string

func (settings boostSettings) Optional(_ context.Context, key string) (string, error) {
	if key != "telegram.group_chat_id" {
		return "", errors.New("unexpected setting")
	}
	return string(settings), nil
}

type boostUsers struct{}

func (boostUsers) UserByID(context.Context, string) (model.User, error) {
	return model.User{TelegramID: 42}, nil
}

type boostTelegram struct {
	boosts []telegram.ChatBoost
	err    error
	calls  int
	chat   string
	user   int64
}

func (provider *boostTelegram) GetUserChatBoosts(_ context.Context, chat string, user int64) (telegram.UserChatBoosts, error) {
	provider.calls++
	provider.chat, provider.user = chat, user
	return telegram.UserChatBoosts{Boosts: provider.boosts}, provider.err
}

func TestActivityBoostAdapterCountsOnlyDistinctActiveBoosts(t *testing.T) {
	now := time.Now().Unix()
	provider := &boostTelegram{boosts: []telegram.ChatBoost{
		{BoostID: "first", AddDate: now - 10, ExpirationDate: now + 3600},
		{BoostID: "first", AddDate: now - 10, ExpirationDate: now + 3600},
		{BoostID: "second", AddDate: now - 10, ExpirationDate: now + 3600},
		{BoostID: "expired", AddDate: now - 100, ExpirationDate: now - 1},
		{BoostID: "future", AddDate: now + 3600, ExpirationDate: now + 7200},
		{ExpirationDate: now + 3600},
	}}
	adapter := activityBoostAdapter{settings: boostSettings("-100123456"), telegram: provider, users: boostUsers{}}
	status, err := adapter.GroupBoost(context.Background(), "member")
	if err != nil || status.State != "boosted" || status.Count == nil || *status.Count != 2 || status.BoostURL == nil || *status.BoostURL != "https://t.me/boost?c=123456" {
		t.Fatalf("boost status = %+v, %v", status, err)
	}
	if provider.chat != "-100123456" || provider.user != 42 || provider.calls != 1 {
		t.Fatalf("lookup = %+v", provider)
	}
}

func TestActivityBoostAdapterUnavailableAndUnboostedStates(t *testing.T) {
	for _, test := range []struct {
		name, group string
		err         error
		unavailable bool
	}{
		{"no boosts", "-100123456", nil, false},
		{"missing configuration", "", nil, true},
		{"basic group", "-123", nil, true},
		{"channel username not configured group ID", "@channel", nil, true},
		{"no administrator rights", "-100123456", errors.New("not administrator"), true},
		{"provider failure", "-100123456", errors.New("timeout"), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := &boostTelegram{err: test.err}
			adapter := activityBoostAdapter{settings: boostSettings(test.group), telegram: provider, users: boostUsers{}}
			status, err := adapter.GroupBoost(context.Background(), "member")
			if test.unavailable {
				if err == nil || status.State != "unavailable" || status.Count != nil {
					t.Fatalf("failure = %+v, %v", status, err)
				}
			} else if err != nil || status.State != "required" || status.Count == nil || *status.Count != 0 {
				t.Fatalf("unboosted = %+v, %v", status, err)
			}
		})
	}
}
