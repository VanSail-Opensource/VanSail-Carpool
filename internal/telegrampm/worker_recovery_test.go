package telegrampm

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestPMWorkerAmbiguousTopicAndCardDoNotRecreateAutomatically(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"topic", "profile", "interrupted topic"} {
		w, r, s := newWorkerFixture()
		switch phase {
		case "topic":
			s.createErr = errors.New("request result unknown")
		case "profile":
			s.profileErr = errors.New("request result unknown")
		default:
			r.operation.Receipt.Status = "processing"
			r.conversation.TopicState, r.conversation.TopicOperationID = "creating", r.operation.Receipt.ID
			item := r.items["topic"]
			item.Status = providerops.StatusProcessing
			r.items["topic"] = item
		}
		if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{Attempts: 1}); err != nil || r.operation.Receipt.Status != "pending_review" || s.copies != 0 {
			t.Fatalf("phase=%s status=%s err=%v", phase, r.operation.Receipt.Status, err)
		}
		if phase == "profile" && r.conversation.ProfileState != "pending_review" {
			t.Fatal("profile certainty not preserved")
		}
		if phase != "profile" && r.conversation.TopicState != "pending_review" {
			t.Fatal("topic certainty not preserved")
		}
	}
}

func TestPMWorkerRateLimitRetriesOnlyKnownRejectedSend(t *testing.T) {
	t.Parallel()
	w, r, s := newWorkerFixture()
	s.copyErr = &telegram.APIError{HTTPStatus: 429, ErrorCode: 429, RetryAfter: 3}
	if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{Attempts: 1}); err == nil || r.items["relay"].Status != providerops.StatusQueued {
		t.Fatalf("rate limit state=%s err=%v", r.items["relay"].Status, err)
	}
	s.copyErr = nil
	if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{Attempts: 2}); err != nil || s.creates != 1 || s.profiles != 1 || s.copies != 2 {
		t.Fatalf("retry repeated phases: %v %+v", err, s)
	}
}

func TestPMWorkerRepairValidatesExistingCardThread(t *testing.T) {
	t.Parallel()
	for _, wrongThread := range []bool{false, true} {
		w, r, s := newWorkerFixture()
		r.operation.Receipt.Kind = providerops.KindTelegramPMRepair
		r.operation.ActorUserID = "admin"
		r.conversation.TopicState = "pending_review"
		r.items = map[string]providerops.Item{"repair": {Key: "repair", TargetType: "pm_topic_recovery", TargetID: "conversation:90:91", Status: providerops.StatusQueued}}
		if wrongThread {
			s.editResult = telegram.Message{MessageID: 91, MessageThreadID: 92, Chat: telegram.Chat{ID: -100123}}
		}
		if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{Attempts: 1}); err != nil {
			t.Fatal(err)
		}
		if wrongThread {
			if r.conversation.TopicID != 0 || r.operation.Receipt.Status != "failed" {
				t.Fatal("wrong thread attached")
			}
		} else if r.conversation.TopicID != 90 || r.conversation.ProfileMessageID != 91 || r.operation.Receipt.Status != "succeeded" {
			t.Fatal("valid card not attached")
		}
	}
}
