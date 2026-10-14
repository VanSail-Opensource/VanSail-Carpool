package app

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func groupMemberTagState(ctx context.Context, client *telegram.Client, chatID string, userID int64) (accounts.GroupMemberTag, error) {
	member, err := client.GetChatMember(ctx, chatID, userID)
	if err != nil {
		return accounts.GroupMemberTag{}, err
	}
	state := accounts.GroupMemberTag{GroupJoined: member.Present(), Tag: member.Tag}
	if !state.GroupJoined {
		state.ReasonCode = "GROUP_NOT_JOINED"
		return state, nil
	}
	if member.Status != "member" && member.Status != "restricted" {
		state.ReasonCode = "GROUP_TAG_ROLE_UNSUPPORTED"
		return state, nil
	}
	if member.Status == "restricted" && !member.CanEditTag {
		state.ReasonCode = "GROUP_TAG_EDIT_RESTRICTED"
		return state, nil
	}
	bot, err := client.GetMe(ctx)
	if err != nil {
		return state, err
	}
	admin, err := client.GetChatMember(ctx, chatID, bot.ID)
	if err != nil {
		return state, err
	}
	state.Editable = admin.Status == "administrator" && admin.CanManageTags
	if !state.Editable {
		state.ReasonCode = "GROUP_TAG_PERMISSION_REQUIRED"
	}
	return state, nil
}

// GroupMemberTag retrieves live membership and bot rights within the queue.
func (a telegramAdapter) GroupMemberTag(ctx context.Context, chatID string, userID int64) (accounts.GroupMemberTag, error) {
	return upstreamqueue.Do(ctx, a.client.queue, func(callCtx context.Context) (accounts.GroupMemberTag, error) {
		return groupMemberTagState(callCtx, a.client.client, chatID, userID)
	})
}

// SetGroupMemberTag rechecks membership before the queued mutation executes.
func (a telegramAdapter) SetGroupMemberTag(ctx context.Context, chatID string, userID int64, tag string) error {
	return upstreamqueue.Execute(ctx, a.client.queue, func(callCtx context.Context) error {
		state, err := groupMemberTagState(callCtx, a.client.client, chatID, userID)
		if err != nil {
			return err
		}
		if !state.Editable {
			return accounts.ErrGroupTagUnavailable
		}
		return a.client.client.SetChatMemberTag(callCtx, chatID, userID, tag)
	})
}
