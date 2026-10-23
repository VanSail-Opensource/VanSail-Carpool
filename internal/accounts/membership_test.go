package accounts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestCheckCommunityMembershipUsesCanonicalFactsAndActiveCombo(t *testing.T) {
	t.Parallel()

	user := model.User{ID: "user-1", TelegramID: 42, OnboardingState: "complete"}
	repository := &accountsRepository{user: user, activeCombo: true}
	telegram := &accountsTelegram{memberships: map[string]bool{"-1001": true, "-1002": false}}
	membership, err := newAccountsServiceForTest(repository, &accountsValidator{}, telegram, &accountsProxy{}, validAccountsSettings(), 1).
		CheckCommunityMembership(context.Background(), user)
	if err != nil {
		t.Fatalf("CheckCommunityMembership(): %v", err)
	}
	if !membership.ActiveCombo || !membership.GroupJoined || membership.ChannelJoined || membership.User.OnboardingState != "complete" {
		t.Fatalf("community membership = %+v", membership)
	}
}

func TestCreateCommunityInviteIsPerSpaceAndEntitlementGated(t *testing.T) {
	t.Parallel()

	user := model.User{ID: "user-1", TelegramID: 42, OnboardingState: "complete"}
	tests := []struct {
		name        string
		space       CommunitySpace
		active      bool
		joined      bool
		want        error
		wantInvites int
		wantChat    string
	}{
		{name: "eligible group", space: CommunityGroup, active: true, wantInvites: 1, wantChat: "-1001"},
		{name: "eligible channel", space: CommunityChannel, active: true, wantInvites: 1, wantChat: "-1002"},
		{name: "already joined", space: CommunityGroup, active: true, joined: true, want: ErrCommunityAlreadyJoined},
		{name: "inactive group combo", space: CommunityGroup, want: ErrActiveComboRequired},
		{name: "inactive channel combo", space: CommunityChannel, want: ErrActiveComboRequired},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &accountsRepository{user: user, activeCombo: test.active}
			telegram := &accountsTelegram{memberships: map[string]bool{"-1001": test.joined, "-1002": test.joined}}
			service := newAccountsServiceForTest(repository, &accountsValidator{}, telegram, &accountsProxy{}, validAccountsSettings(), 1)
			link, expiresAt, err := service.CreateCommunityInvite(context.Background(), user, test.space)
			if !errors.Is(err, test.want) {
				t.Fatalf("CreateCommunityInvite() error = %v, want %v", err, test.want)
			}
			if len(telegram.inviteNames) != test.wantInvites || len(telegram.inviteChatIDs) != test.wantInvites {
				t.Fatalf("created invite count = %d/%d, want %d", len(telegram.inviteNames), len(telegram.inviteChatIDs), test.wantInvites)
			}
			if test.want == nil && (link == "" || telegram.inviteChatIDs[0] != test.wantChat || !expiresAt.Equal(service.now().UTC().Add(30*time.Minute))) {
				t.Fatalf("invite = %q, chats = %v, expires = %s", link, telegram.inviteChatIDs, expiresAt)
			}
		})
	}
}

func TestHandleSignedJoinRequestRechecksAndDeclinesExpiredEntitlement(t *testing.T) {
	t.Parallel()

	const telegramID, chatID = int64(42), int64(-1001)
	user := model.User{ID: "user-1", TelegramID: telegramID, OnboardingState: "complete"}
	tests := []struct {
		name        string
		active      bool
		want        error
		wantApprove int
		wantDecline int
	}{
		{name: "active approval", active: true, wantApprove: 1},
		{name: "lapsed entitlement", want: ErrActiveComboRequired, wantDecline: 1},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &accountsRepository{user: user, activeCombo: test.active}
			telegram := &accountsTelegram{}
			service := newAccountsServiceForTest(repository, &accountsValidator{}, telegram, &accountsProxy{}, validAccountsSettings(), 1)
			expiresAt := service.now().UTC().Add(time.Hour)
			name, err := service.signedInviteName(context.Background(), telegramID, chatID, expiresAt)
			if err != nil {
				t.Fatalf("signedInviteName(): %v", err)
			}
			err = service.HandleSignedJoinRequest(context.Background(), telegramID, chatID, "invite", name, expiresAt)
			if !errors.Is(err, test.want) || telegram.approveCalls != test.wantApprove || telegram.declineCalls != test.wantDecline || len(telegram.revokedLinks) != 1 {
				t.Fatalf("HandleSignedJoinRequest() = %v, calls approve/decline/revoke %d/%d/%d", err, telegram.approveCalls, telegram.declineCalls, len(telegram.revokedLinks))
			}
		})
	}
}
