package telegrampm

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestPMWorkerPersistsTopicBeforeCopyAndDoesNotReplaySuccess(t *testing.T) {
	t.Parallel()
	w, r, s := newWorkerFixture()
	if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{Attempts: 1}); err != nil {
		t.Fatal(err)
	}
	if s.creates != 1 || s.profiles != 1 || s.copies != 1 || r.operation.Receipt.Status != "succeeded" || s.request.ChatID != -100123 || s.request.MessageThreadID != 60 || s.request.MessageID != 4 {
		t.Fatalf("routing=%+v receipt=%+v", s.request, r.operation.Receipt)
	}
	if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{Attempts: 2}); err != nil || s.copies != 1 {
		t.Fatalf("duplicate success=%v copies=%d", err, s.copies)
	}
}

func TestPMWorkerRechecksModerationInsideQueuedCopy(t *testing.T) {
	t.Parallel()
	for _, blocked := range []bool{false, true} {
		w, r, s := newWorkerFixture()
		s.beforeCopy = func() { r.conversation.Muted = true; r.conversation.Blocked = blocked }
		if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{Attempts: 1}); err != nil {
			t.Fatal(err)
		}
		if blocked {
			if s.copies != 0 || r.operation.Receipt.Status != "failed" {
				t.Fatal("late block did not stop copy")
			}
		} else if s.copies != 1 || !s.request.DisableNotification {
			t.Fatal("late mute was not applied")
		}
	}
}

func TestPMWorkerUnknownCopyOrRestartRequiresReview(t *testing.T) {
	t.Parallel()
	for _, interrupted := range []bool{false, true} {
		w, r, s := newWorkerFixture()
		if interrupted {
			r.operation.Receipt.Status = "processing"
			r.conversation.TopicID, r.conversation.ProfileMessageID = 50, 51
			r.conversation.TopicState = "ready"
			for _, key := range []string{"topic", "profile"} {
				item := r.items[key]
				item.Status = providerops.StatusSucceeded
				r.items[key] = item
			}
			item := r.items["relay"]
			item.Status = providerops.StatusProcessing
			r.items["relay"] = item
		} else {
			s.copyErr = errors.New("connection closed after request")
		}
		if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{Attempts: 1}); err != nil {
			t.Fatal(err)
		}
		if r.operation.Receipt.Status != "pending_review" || r.operation.Receipt.ErrorCode == nil || *r.operation.Receipt.ErrorCode != "PM_DELIVERY_UNCERTAIN" {
			t.Fatalf("receipt=%+v", r.operation.Receipt)
		}
		copies := s.copies
		if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{Attempts: 2}); err != nil || s.copies != copies {
			t.Fatal("uncertain copy replayed")
		}
	}
}

func TestPMWorkerOnlyMissingTopicRecreatesIt(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		description string
		missing     bool
	}{{"Bad Request: message thread not found", true}, {"Bad Request: TOPIC_CLOSED", false}, {"Bad Request: not enough rights", false}} {
		w, r, s := newWorkerFixture()
		s.copyErr = &telegram.APIError{HTTPStatus: 400, ErrorCode: 400, Description: test.description}
		err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{Attempts: 1})
		if test.missing {
			if err == nil || r.resets != 1 || r.conversation.TopicState != "new" {
				t.Fatalf("missing reset=%d state=%s err=%v", r.resets, r.conversation.TopicState, err)
			}
			s.copyErr = nil
			if err := w.HandleProviderOperation(context.Background(), r.operation, model.OutboxJob{Attempts: 2}); err != nil || s.creates != 2 || r.operation.Receipt.Status != "succeeded" {
				t.Fatalf("recovery=%v creates=%d", err, s.creates)
			}
		} else if err != nil || r.resets != 0 || s.creates != 1 || r.operation.Receipt.Status != "failed" {
			t.Fatalf("wrong recreate=%d creates=%d err=%v", r.resets, s.creates, err)
		}
	}
}
