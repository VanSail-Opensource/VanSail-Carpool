package accounts

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"strings"
	"unicode/utf8"
)

// ErrGroupTagUnavailable means Telegram membership or permissions prevent edits.
var ErrGroupTagUnavailable = errors.New("group member tag cannot be edited")

// ErrInvalidGroupTag denotes invalid Unicode or excessive tag length.
var ErrInvalidGroupTag = errors.New("invalid group member tag")

// GroupMemberTag is a live Telegram projection, never a local copy.
type GroupMemberTag struct {
	GroupJoined bool   `json:"groupJoined"`
	Editable    bool   `json:"editable"`
	Tag         string `json:"tag"`
	ReasonCode  string `json:"reasonCode"`
}

type groupTagClient interface {
	GroupMemberTag(context.Context, string, int64) (GroupMemberTag, error)
	SetGroupMemberTag(context.Context, string, int64, string) error
}

// GroupMemberTag checks canonical group membership without requiring a combo.
func (s *Service) GroupMemberTag(ctx context.Context, user model.User) (GroupMemberTag, error) {
	_, chatID, err := s.communityChatID(ctx, CommunityGroup)
	if err != nil {
		return GroupMemberTag{}, err
	}
	client, ok := s.telegram.(groupTagClient)
	if !ok {
		return GroupMemberTag{}, ErrGroupTagUnavailable
	}
	return client.GroupMemberTag(ctx, chatID, user.TelegramID)
}

// SetGroupMemberTag delegates fresh execution-time permission checks to the queue.
func (s *Service) SetGroupMemberTag(ctx context.Context, user model.User, tag string) (GroupMemberTag, error) {
	tag = strings.TrimSpace(tag)
	if !utf8.ValidString(tag) || utf8.RuneCountInString(tag) > 16 {
		return GroupMemberTag{}, ErrInvalidGroupTag
	}
	_, chatID, err := s.communityChatID(ctx, CommunityGroup)
	if err != nil {
		return GroupMemberTag{}, err
	}
	client, ok := s.telegram.(groupTagClient)
	if !ok {
		return GroupMemberTag{}, ErrGroupTagUnavailable
	}
	if err := client.SetGroupMemberTag(ctx, chatID, user.TelegramID, tag); err != nil {
		return GroupMemberTag{}, err
	}
	return client.GroupMemberTag(ctx, chatID, user.TelegramID)
}
