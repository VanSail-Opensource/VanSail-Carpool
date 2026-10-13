package telegrampm

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/integrations/telegram"
	"github.com/txyyddss/Remna-User-Panel/internal/providerops"
)

func (w *Worker) profile(ctx context.Context, run execution, item providerops.Item, interrupted bool) (phaseResult, error) {
	conversation, err := w.Repository.PMConversation(ctx, run.conversationID)
	if err != nil {
		return phaseResult{}, err
	}
	if conversation.TopicState != "ready" || conversation.TopicID <= 1 {
		return phaseResult{status: providerops.StatusPendingReview, code: "PM_TOPIC_UNCERTAIN"}, nil
	}
	facts := ProfileFacts{OnboardingState: "complete"}
	if w.Profiles != nil {
		facts, err = w.Profiles.PMProfileFacts(ctx, conversation.UserID)
		if err != nil {
			return phaseResult{}, err
		}
	}
	if (interrupted || conversation.ProfileState == "pending_review") && conversation.ProfileMessageID == 0 {
		return phaseResult{status: providerops.StatusPendingReview, code: "PM_PROFILE_UNCERTAIN"}, nil
	}
	if conversation.ProfileMessageID > 0 {
		var started atomic.Bool
		messageID := conversation.ProfileMessageID
		_, callErr := w.Sender.EditPMProfile(ctx, func(callCtx context.Context) (telegram.TopicProfileRequest, error) {
			current, err := w.current(callCtx, run)
			if err != nil {
				return telegram.TopicProfileRequest{}, err
			}
			if current.ProfileMessageID != messageID {
				return telegram.TopicProfileRequest{}, ErrTopicUncertain
			}
			started.Store(true)
			return Profile(current, facts, w.PublicBaseURL, w.Timezone), nil
		})
		if !profileMissing(callErr) {
			return w.callResult(ctx, run, item, callErr, started.Load(), false, strconv.FormatInt(messageID, 10), "PM_PROFILE_UNCERTAIN")
		}
		if err := w.Repository.ClearPMProfile(ctx, conversation.ID, messageID); err != nil {
			return phaseResult{}, err
		}
	}
	uncertain, err := w.Repository.PMProfileUncertain(ctx, conversation.ID, run.operation.Receipt.ID)
	if err != nil {
		return phaseResult{}, err
	}
	if uncertain {
		return phaseResult{status: providerops.StatusPendingReview, code: "PM_PROFILE_UNCERTAIN"}, nil
	}
	var started atomic.Bool
	var topicID atomic.Int64
	messageID, callErr := w.Sender.PublishPMProfile(ctx, func(callCtx context.Context) (telegram.TopicProfileRequest, error) {
		current, err := w.current(callCtx, run)
		if err != nil {
			return telegram.TopicProfileRequest{}, err
		}
		if current.TopicState != "ready" || current.TopicID <= 1 || current.ProfileMessageID != 0 {
			return telegram.TopicProfileRequest{}, ErrTopicUncertain
		}
		topicID.Store(current.TopicID)
		started.Store(true)
		return Profile(current, facts, w.PublicBaseURL, w.Timezone), nil
	})
	if topicMissing(callErr) && run.job.Attempts < 10 {
		if err := w.Repository.ResetMissingPMTopic(ctx, conversation.ID, run.operation.Receipt.ID, topicID.Load(), time.Now().UTC()); err != nil {
			return phaseResult{}, err
		}
		return phaseResult{}, callErr
	}
	if callErr == nil {
		persist, cancel := completionContext(ctx)
		defer cancel()
		if err := w.Repository.SavePMProfile(persist, conversation.ID, topicID.Load(), messageID, time.Now().UTC()); err != nil {
			return phaseResult{}, err
		}
	}
	return w.callResult(ctx, run, item, callErr, started.Load(), true, strconv.FormatInt(messageID, 10), "PM_PROFILE_UNCERTAIN")
}
