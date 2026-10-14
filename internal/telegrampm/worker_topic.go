package telegrampm

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (w *Worker) topic(ctx context.Context, run execution, item providerops.Item, interrupted bool) (phaseResult, error) {
	conversation, err := w.Repository.PMConversation(ctx, run.conversationID)
	if err != nil {
		return phaseResult{}, err
	}
	if conversation.TopicID > 1 && conversation.TopicState == "ready" {
		return phaseResult{status: providerops.StatusSucceeded, reference: strconv.FormatInt(conversation.TopicID, 10)}, nil
	}
	if conversation.TopicState == "pending_review" {
		return phaseResult{status: providerops.StatusPendingReview, code: "PM_TOPIC_UNCERTAIN"}, nil
	}
	if interrupted && conversation.TopicState == "creating" && conversation.TopicOperationID == run.operation.Receipt.ID {
		if err := w.Repository.ReleasePMTopic(ctx, conversation.ID, run.operation.Receipt.ID, true, time.Now().UTC()); err != nil {
			return phaseResult{}, err
		}
		return phaseResult{status: providerops.StatusPendingReview, code: "PM_TOPIC_UNCERTAIN"}, nil
	}
	claimed, err := w.Repository.ClaimPMTopic(ctx, conversation.ID, run.operation.Receipt.ID, time.Now().UTC())
	if err != nil {
		return phaseResult{}, err
	}
	if !claimed {
		return w.callResult(ctx, run, item, fmt.Errorf("PM topic creation busy"), false, true, "", "PM_TOPIC_UNCERTAIN")
	}
	var started atomic.Bool
	topic, callErr := w.Sender.CreatePMTopic(ctx, TopicName(conversation), func(callCtx context.Context) (int64, error) {
		current, err := w.current(callCtx, run)
		if err != nil {
			return 0, err
		}
		if current.TopicState != "creating" || current.TopicOperationID != run.operation.Receipt.ID {
			return 0, ErrTopicUncertain
		}
		started.Store(true)
		return current.ChatID, nil
	})
	persist, cancel := completionContext(ctx)
	defer cancel()
	if callErr == nil {
		if err := w.Repository.SavePMTopic(persist, conversation.ID, run.operation.Receipt.ID, topic.MessageThreadID, time.Now().UTC()); err != nil {
			return phaseResult{}, err
		}
	} else {
		uncertain := started.Load() && !definiteRejection(callErr)
		if err := w.Repository.ReleasePMTopic(persist, conversation.ID, run.operation.Receipt.ID, uncertain, time.Now().UTC()); err != nil {
			return phaseResult{}, err
		}
	}
	return w.callResult(ctx, run, item, callErr, started.Load(), true, strconv.FormatInt(topic.MessageThreadID, 10), "PM_TOPIC_UNCERTAIN")
}
