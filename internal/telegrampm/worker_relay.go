package telegrampm

import (
	"context"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func messageReference(value string) (int64, int64, error) {
	chat, message, ok := strings.Cut(value, ":")
	if !ok {
		return 0, 0, ErrBlocked
	}
	chatID, err := strconv.ParseInt(chat, 10, 64)
	if err != nil {
		return 0, 0, err
	}
	messageID, err := strconv.ParseInt(message, 10, 64)
	if err != nil || chatID == 0 || messageID <= 0 {
		return 0, 0, ErrBlocked
	}
	return chatID, messageID, nil
}

func (w *Worker) relay(ctx context.Context, run execution, item providerops.Item, interrupted bool) (phaseResult, error) {
	if interrupted {
		return phaseResult{status: providerops.StatusPendingReview, code: "PM_DELIVERY_UNCERTAIN"}, nil
	}
	sourceChat, messageID, err := messageReference(item.TargetID)
	if err != nil {
		return phaseResult{}, err
	}
	var started atomic.Bool
	var topicID atomic.Int64
	var content *OutboundContent
	if !run.inbound {
		content, err = w.content(ctx, run)
		if err != nil {
			return w.callResult(ctx, run, item, err, false, true, "", "PM_DELIVERY_UNCERTAIN")
		}
		if content != nil && content.Mode == "text" {
			result, callErr := w.Sender.SendPMText(ctx, func(ctx context.Context) (telegram.PMTextRequest, error) {
				request, err := w.outboundText(ctx, run, item.TargetID)
				if err == nil {
					started.Store(true)
				}
				return request, err
			})
			return w.callResult(ctx, run, item, callErr, started.Load(), true, strconv.FormatInt(result, 10), "PM_DELIVERY_UNCERTAIN")
		}
	}
	result, callErr := w.Sender.CopyPMMessage(ctx, func(callCtx context.Context) (telegram.CopyMessageRequest, error) {
		conversation, err := w.current(callCtx, run)
		if err != nil {
			return telegram.CopyMessageRequest{}, err
		}
		if conversation.TopicState != "ready" || conversation.TopicID <= 1 {
			return telegram.CopyMessageRequest{}, ErrTopicUncertain
		}
		request := telegram.CopyMessageRequest{FromChatID: sourceChat, MessageID: messageID, DisableNotification: conversation.Muted}
		if run.inbound {
			if sourceChat != conversation.TelegramID {
				return request, ErrBlocked
			}
			request.ChatID = conversation.ChatID
			request.MessageThreadID = conversation.TopicID
		} else {
			if sourceChat != conversation.ChatID {
				return request, ErrBlocked
			}
			request.ChatID = conversation.TelegramID
			fresh, err := w.content(callCtx, run)
			if err != nil {
				return request, err
			}
			if content != nil && fresh == nil {
				return request, model.ErrPMContentInvalid
			}
			if fresh != nil && fresh.Mode == "caption" {
				caption := fresh.Caption
				if caption != "" {
					caption += "\n\n"
				}
				caption += fresh.Footer
				request.Caption = &caption
				request.CaptionEntities = fresh.CaptionEntities
				request.ShowCaptionAboveMedia = fresh.ShowCaptionAboveMedia
			}
		}
		topicID.Store(conversation.TopicID)
		started.Store(true)
		return request, nil
	})
	if run.inbound && topicMissing(callErr) && run.job.Attempts < 10 {
		if err := w.Repository.ResetMissingPMTopic(ctx, run.conversationID, run.operation.Receipt.ID, topicID.Load(), time.Now().UTC()); err != nil {
			return phaseResult{}, err
		}
		return phaseResult{}, callErr
	}
	return w.callResult(ctx, run, item, callErr, started.Load(), true, strconv.FormatInt(result, 10), "PM_DELIVERY_UNCERTAIN")
}
