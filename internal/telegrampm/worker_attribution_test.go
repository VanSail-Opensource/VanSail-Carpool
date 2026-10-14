package telegrampm

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type contentWorkerRepo struct {
	*workerRepo
	content    OutboundContent
	contentErr error
}

func (r *contentWorkerRepo) PMRelayContent(context.Context, string, time.Time) (json.RawMessage, bool, error) {
	b, _ := json.Marshal(r.content)
	return b, true, r.contentErr
}

type contentWorkerSender struct {
	*workerSender
	texts   []telegram.PMTextRequest
	textErr error
}

func (s *contentWorkerSender) SendPMText(ctx context.Context, guard func(context.Context) (telegram.PMTextRequest, error)) (int64, error) {
	r, err := guard(ctx)
	if err != nil {
		return 0, err
	}
	s.texts = append(s.texts, r)
	return 63, s.textErr
}

func attributionFixture(mode string) (*Worker, *contentWorkerRepo, *contentWorkerSender) {
	w, r, s := newWorkerFixture()
	r.conversation.TopicState = "ready"
	r.conversation.TopicID = 12
	r.conversation.ProfileMessageID = 61
	r.conversation.Muted = true
	r.operation.ActorUserID = "admin"
	for _, key := range []string{"topic", "profile"} {
		item := r.items[key]
		item.Status = providerops.StatusSucceeded
		r.items[key] = item
	}
	item := r.items["relay"]
	item.TargetType = "pm_outbound_message"
	item.TargetID = "-100123:4"
	r.items["relay"] = item
	repo := &contentWorkerRepo{workerRepo: r, content: OutboundContent{Mode: mode, Text: "Message 👋", Caption: "Caption", Footer: "By @admin_ops", Entities: []telegram.MessageEntity{{Type: "bold", Offset: 0, Length: 7}}}}
	sender := &contentWorkerSender{workerSender: s}
	w.Repository = repo
	w.Sender = sender
	return w, repo, sender
}

func TestAttributedTextAndCaptionDelivery(t *testing.T) {
	for _, mode := range []string{"text", "caption"} {
		t.Run(mode, func(t *testing.T) {
			w, r, s := attributionFixture(mode)
			if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{}); err != nil {
				t.Fatal(err)
			}
			if mode == "text" && (s.copies != 0 || len(s.texts) != 1 || s.texts[0].Text != "Message 👋\n\nBy @admin_ops" || len(s.texts[0].Entities) != 1 || !s.texts[0].DisableNotification) {
				t.Fatalf("text: %+v", s.texts)
			}
			if mode == "caption" && (s.copies != 1 || s.request.Caption == nil || *s.request.Caption != "Caption\n\nBy @admin_ops") {
				t.Fatalf("caption: %+v", s.request)
			}
			if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{}); err != nil {
				t.Fatal(err)
			}
			if s.copies > 1 || len(s.texts) > 1 {
				t.Fatal("replayed settled delivery")
			}
		})
	}
}

func TestFooterPhaseRetriesDoNotReplayOriginalCopy(t *testing.T) {
	w, r, s := attributionFixture("copy-footer")
	r.items["footer"] = providerops.Item{Key: "footer", TargetType: "pm_outbound_footer", TargetID: "-100123:4", Status: providerops.StatusQueued}
	s.textErr = &telegram.APIError{ErrorCode: 429, Description: "limited"}
	if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{Attempts: 1}); err == nil {
		t.Fatal("expected retry")
	}
	if s.copies != 1 || len(s.texts) != 1 || s.texts[0].ReplyParameters.MessageID != 62 {
		t.Fatal("footer not linked to copy")
	}
	s.textErr = nil
	if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{Attempts: 2}); err != nil {
		t.Fatal(err)
	}
	if s.copies != 1 || len(s.texts) != 2 {
		t.Fatal("copy was replayed")
	}
}

func TestExpiredAndAmbiguousAttributionNeverReplay(t *testing.T) {
	w, r, s := attributionFixture("text")
	r.contentErr = model.ErrPMContentExpired
	if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	if r.operation.Receipt.ErrorCode == nil || *r.operation.Receipt.ErrorCode != "PM_CONTENT_EXPIRED" || len(s.texts) != 0 || s.copies != 0 {
		t.Fatal("expired content sent or treated as legacy")
	}
	w, r, s = attributionFixture("text")
	s.textErr = errors.New("response lost")
	if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	if r.operation.Receipt.Status != "pending_review" {
		t.Fatal("ambiguous text not reviewed")
	}
	if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{}); err != nil {
		t.Fatal(err)
	}
	if len(s.texts) != 1 {
		t.Fatal("ambiguous text replayed")
	}
}
