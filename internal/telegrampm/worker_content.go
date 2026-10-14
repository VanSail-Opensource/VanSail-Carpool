package telegrampm

import (
	"context"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type contentRepository interface {
	PMRelayContent(context.Context, string, time.Time) (json.RawMessage, bool, error)
}

func (w *Worker) content(ctx context.Context, run execution) (*OutboundContent, error) {
	reader, ok := w.Repository.(contentRepository)
	if !ok {
		return nil, nil
	} // Reference-only repositories are legacy senders.
	data, found, err := reader.PMRelayContent(ctx, run.operation.Receipt.ID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	var content OutboundContent
	if json.Unmarshal(data, &content) != nil || content.Footer == "" || telegram.UTF16Length(content.Footer) > 512 {
		return nil, model.ErrPMContentInvalid
	}
	switch content.Mode {
	case "text":
		if content.Text == "" || telegram.UTF16Length(content.Text+"\n\n"+content.Footer) > 4096 {
			return nil, model.ErrPMContentInvalid
		}
	case "caption":
		if telegram.UTF16Length(content.Caption+"\n\n"+content.Footer) > 1024 {
			return nil, model.ErrPMContentInvalid
		}
	case "copy-footer":
	default:
		return nil, model.ErrPMContentInvalid
	}
	return &content, nil
}

func (w *Worker) outboundText(ctx context.Context, run execution, itemID string) (telegram.PMTextRequest, error) {
	conversation, err := w.current(ctx, run)
	if err != nil {
		return telegram.PMTextRequest{}, err
	}
	if conversation.TopicState != "ready" || conversation.TopicID <= 1 {
		return telegram.PMTextRequest{}, ErrTopicUncertain
	}
	source, _, err := messageReference(itemID)
	if err != nil || source != conversation.ChatID || run.inbound {
		return telegram.PMTextRequest{}, ErrBlocked
	}
	content, err := w.content(ctx, run)
	if err != nil {
		return telegram.PMTextRequest{}, err
	}
	if content == nil || content.Mode != "text" {
		return telegram.PMTextRequest{}, model.ErrPMContentInvalid
	}
	return telegram.PMTextRequest{ChatID: conversation.TelegramID, Text: content.Text + "\n\n" + content.Footer, Entities: content.Entities, DisableNotification: conversation.Muted}, nil
}
