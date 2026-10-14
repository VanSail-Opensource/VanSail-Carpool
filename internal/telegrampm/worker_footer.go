package telegrampm

import (
	"context"
	"strconv"
	"sync/atomic"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (w *Worker) footer(ctx context.Context, run execution, item providerops.Item, interrupted bool) (phaseResult, error) {
	if interrupted {
		return phaseResult{status: providerops.StatusPendingReview, code: "PM_DELIVERY_UNCERTAIN"}, nil
	}
	var started atomic.Bool
	result, callErr := w.Sender.SendPMText(ctx, func(ctx context.Context) (telegram.PMTextRequest, error) {
		conversation, err := w.current(ctx, run)
		if err != nil {
			return telegram.PMTextRequest{}, err
		}
		source, _, err := messageReference(item.TargetID)
		if err != nil || run.inbound || source != conversation.ChatID {
			return telegram.PMTextRequest{}, ErrBlocked
		}
		content, err := w.content(ctx, run)
		if err != nil {
			return telegram.PMTextRequest{}, err
		}
		text := ""
		if content != nil {
			text = content.Footer
		} else {
			actor, err := w.Repository.UserByID(ctx, run.operation.ActorUserID)
			if err != nil {
				return telegram.PMTextRequest{}, err
			}
			text = adminFooter(telegram.User{ID: actor.TelegramID, Username: actor.TelegramUsername, FirstName: actor.TelegramFirstName, LastName: actor.TelegramLastName})
		}
		items, err := w.Repository.ProviderOperationItems(ctx, run.operation.Receipt.ID)
		if err != nil {
			return telegram.PMTextRequest{}, err
		}
		var reply int64
		for _, prior := range items {
			if prior.Key == "relay" && prior.Status == providerops.StatusSucceeded {
				reply, _ = strconv.ParseInt(prior.ProviderReference, 10, 64)
			}
		}
		if reply <= 0 {
			return telegram.PMTextRequest{}, ErrBlocked
		}
		started.Store(true)
		return telegram.PMTextRequest{ChatID: conversation.TelegramID, Text: text, DisableNotification: conversation.Muted, ReplyParameters: &telegram.PMReplyParameters{MessageID: reply}}, nil
	})
	return w.callResult(ctx, run, item, callErr, started.Load(), true, strconv.FormatInt(result, 10), "PM_DELIVERY_UNCERTAIN")
}
