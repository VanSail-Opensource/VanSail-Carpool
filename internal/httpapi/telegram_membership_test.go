package httpapi

import (
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
)

func TestTelegramMembershipJoined(t *testing.T) {
	t.Parallel()
	member := telegram.ChatMember{Status: "member", User: telegram.User{ID: 42}}
	left := telegram.ChatMember{Status: "left", User: telegram.User{ID: 42}}
	tests := []struct {
		name    string
		groupID int64
		update  *telegram.ChatMemberUpdated
		want    bool
	}{
		{name: "genuine join", groupID: -100, update: membershipUpdate(-100, left, member), want: true},
		{name: "rejoin", groupID: -100, update: membershipUpdate(-100, telegram.ChatMember{Status: "kicked", User: telegram.User{ID: 42}}, member), want: true},
		{name: "promotion", groupID: -100, update: membershipUpdate(-100, member, telegram.ChatMember{Status: "administrator", User: telegram.User{ID: 42}})},
		{name: "departure", groupID: -100, update: membershipUpdate(-100, member, left)},
		{name: "bot", groupID: -100, update: membershipUpdate(-100, left, telegram.ChatMember{Status: "member", User: telegram.User{ID: 42, IsBot: true}})},
		{name: "unrelated chat", groupID: -200, update: membershipUpdate(-100, left, member)},
		{name: "missing configuration", update: membershipUpdate(-100, left, member)},
		{name: "nil update", groupID: -100},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := telegramMembershipJoined(test.update, test.groupID); got != test.want {
				t.Errorf("telegramMembershipJoined() = %t, want %t", got, test.want)
			}
		})
	}
}

func membershipUpdate(chatID int64, oldMember, newMember telegram.ChatMember) *telegram.ChatMemberUpdated {
	return &telegram.ChatMemberUpdated{Chat: telegram.Chat{ID: chatID}, OldChatMember: oldMember, NewChatMember: newMember}
}
