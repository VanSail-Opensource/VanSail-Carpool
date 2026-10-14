package accounts

import (
	"context"
	"errors"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// CommunitySpace identifies one independently joinable Telegram destination.
type CommunitySpace string

const (
	CommunityGroup   CommunitySpace = "group"
	CommunityChannel CommunitySpace = "channel"
)

// CommunityMembership combines canonical Telegram facts with the current
// server-side combo eligibility check.
type CommunityMembership struct {
	ActiveCombo   bool
	GroupJoined   bool
	ChannelJoined bool
	User          model.User
}

// CheckCommunityAccess evaluates only the current active-combo window.
func (s *Service) CheckCommunityAccess(ctx context.Context, user model.User) (bool, error) {
	return s.hasActiveCombo(ctx, user.ID)
}

// CheckCommunityMembership asks Telegram for canonical membership facts and
// evaluates the active-combo window separately from membership history.
func (s *Service) CheckCommunityMembership(ctx context.Context, user model.User) (CommunityMembership, error) {
	refreshed, err := s.CheckMembership(ctx, user)
	if err != nil {
		return CommunityMembership{}, err
	}
	active, err := s.hasActiveCombo(ctx, refreshed.ID)
	if err != nil {
		return CommunityMembership{}, err
	}
	return CommunityMembership{ActiveCombo: active, GroupJoined: refreshed.GroupJoined, ChannelJoined: refreshed.ChannelJoined, User: refreshed}, nil
}

// CheckMembership asks Telegram for canonical state rather than trusting the browser.
func (s *Service) CheckMembership(ctx context.Context, user model.User) (model.User, error) {
	joined := make(map[CommunitySpace]bool, 2)
	for _, space := range []CommunitySpace{CommunityGroup, CommunityChannel} {
		_, chatID, err := s.communityChatID(ctx, space)
		if err != nil {
			return model.User{}, err
		}
		joined[space], err = s.telegram.GetMembership(ctx, chatID, user.TelegramID)
		if err != nil {
			return model.User{}, fmt.Errorf("check %s membership: %w", space, err)
		}
	}
	return s.repository.UpdateMembership(ctx, user.ID, joined[CommunityGroup], joined[CommunityChannel])
}

// RefreshMembershipByTelegramID updates persisted Telegram membership facts.
func (s *Service) RefreshMembershipByTelegramID(ctx context.Context, telegramID int64) (model.User, error) {
	user, err := s.repository.UserByTelegramID(ctx, telegramID)
	if err != nil {
		return model.User{}, err
	}
	return s.CheckMembership(ctx, user)
}

func (s *Service) checkCommunitySpaceMembership(ctx context.Context, user model.User, space CommunitySpace) (model.User, bool, error) {
	_, chatID, err := s.communityChatID(ctx, space)
	if err != nil {
		return model.User{}, false, err
	}
	joined, err := s.telegram.GetMembership(ctx, chatID, user.TelegramID)
	if err != nil {
		return model.User{}, false, fmt.Errorf("check %s membership: %w", space, err)
	}
	groupJoined, channelJoined := user.GroupJoined, user.ChannelJoined
	if space == CommunityGroup {
		groupJoined = joined
	} else {
		channelJoined = joined
	}
	refreshed, err := s.repository.UpdateMembership(ctx, user.ID, groupJoined, channelJoined)
	return refreshed, joined, err
}

func (s *Service) hasActiveCombo(ctx context.Context, userID string) (bool, error) {
	if s.eligibility == nil {
		return false, errors.New("community eligibility is unavailable")
	}
	return s.eligibility.HasActiveCombo(ctx, userID, s.now().UTC())
}

func (space CommunitySpace) valid() bool {
	return space == CommunityGroup || space == CommunityChannel
}
