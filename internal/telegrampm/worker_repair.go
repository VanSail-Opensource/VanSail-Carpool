package telegrampm

import (
	"context"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/integrations/telegram"
	"github.com/txyyddss/Remna-User-Panel/internal/providerops"
	"github.com/txyyddss/Remna-User-Panel/internal/telegramformat"
)

func (w *Worker) repair(ctx context.Context, run execution, item providerops.Item, interrupted bool) (phaseResult, error) {
	parts := strings.Split(item.TargetID, ":")
	if len(parts) != 3 {
		return phaseResult{}, ErrTopicUncertain
	}
	topicID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || topicID <= 1 {
		return phaseResult{}, ErrTopicUncertain
	}
	profileID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || profileID < 0 {
		return phaseResult{}, ErrTopicUncertain
	}
	conversation, err := w.Repository.PMConversation(ctx, run.conversationID)
	if err != nil {
		return phaseResult{}, err
	}
	if interrupted && conversation.TopicOperationID == run.operation.Receipt.ID && conversation.TopicID == topicID && conversation.ProfileMessageID > 0 {
		return phaseResult{status: providerops.StatusSucceeded, reference: strconv.FormatInt(conversation.ProfileMessageID, 10)}, nil
	}
	if interrupted && profileID == 0 {
		return phaseResult{status: providerops.StatusPendingReview, code: "PM_PROFILE_UNCERTAIN"}, nil
	}
	facts := ProfileFacts{OnboardingState: "complete"}
	if w.Profiles != nil {
		facts, err = w.Profiles.PMProfileFacts(ctx, conversation.UserID)
		if err != nil {
			return phaseResult{}, err
		}
	}
	var started atomic.Bool
	guard := func(callCtx context.Context) (telegram.TopicProfileRequest, error) {
		current, err := w.current(callCtx, run)
		if err != nil {
			return telegram.TopicProfileRequest{}, err
		}
		if current.TopicState == "creating" {
			return telegram.TopicProfileRequest{}, ErrTopicUncertain
		}
		current.TopicID, current.ProfileMessageID = topicID, profileID
		started.Store(true)
		profile := Profile(current, facts, w.PublicBaseURL, w.Timezone)
		if profileID > 0 {
			label := "Profile checked at "
			if chinese(current.Locale) {
				label = "资料卡验证时间："
			}
			profile.Text += "\n" + telegramformat.Escape(label+time.Now().UTC().Format(time.RFC3339Nano))
		}
		return profile, nil
	}
	var callErr error
	if profileID > 0 {
		var message telegram.Message
		message, callErr = w.Sender.EditPMProfile(ctx, guard)
		if callErr == nil && (message.Chat.ID != conversation.ChatID || message.MessageThreadID != topicID || message.MessageID != profileID) {
			callErr = ErrTopicUncertain
		}
	} else {
		profileID, callErr = w.Sender.PublishPMProfile(ctx, guard)
	}
	if callErr == nil {
		persist, cancel := completionContext(ctx)
		defer cancel()
		if err := w.Repository.SavePMTopicRepair(persist, conversation.ID, run.operation.Receipt.ID, topicID, profileID, time.Now().UTC()); err != nil {
			return phaseResult{}, err
		}
	}
	return w.callResult(ctx, run, item, callErr, started.Load(), parts[2] == "0", strconv.FormatInt(profileID, 10), "PM_PROFILE_UNCERTAIN")
}
