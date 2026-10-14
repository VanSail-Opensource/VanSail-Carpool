package notifications

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

type senderStub struct {
	calls int
	err   error
}

func (s *senderStub) SendMarkdownV2Message(context.Context, int64, int64, string) error {
	s.calls++
	return s.err
}

func TestWorkerDeliversOneSuccessfulJob(t *testing.T) {
	payload := notificationFixture(jobpayload.UserEventExpiration, "en", map[string]string{
		FactCombo: "Pro", FactExpired: "2026-08-20T00:00:00Z",
	})
	encoded, err := jobpayload.EncodeUserNotification(payload)
	if err != nil {
		t.Fatal(err)
	}
	sender := &senderStub{}
	worker := NewWorker(sender, nil, time.UTC)
	if err := worker.HandleOutbox(context.Background(), model.OutboxJob{
		Kind: jobpayload.UserNotificationKind, Payload: encoded,
	}); err != nil {
		t.Fatal(err)
	}
	if sender.calls != 1 {
		t.Fatalf("sender calls = %d", sender.calls)
	}
}

func TestWorkerKeepsTelegramFailuresRetryable(t *testing.T) {
	payload := notificationFixture(jobpayload.UserEventExpiration, "en", map[string]string{
		FactCombo: "Pro", FactExpired: "2026-08-20T00:00:00Z",
	})
	encoded, err := jobpayload.EncodeUserNotification(payload)
	if err != nil {
		t.Fatal(err)
	}
	sender := &senderStub{err: errors.New("temporary Telegram failure")}
	worker := NewWorker(sender, nil, time.UTC)
	err = worker.HandleOutbox(context.Background(), model.OutboxJob{Kind: jobpayload.UserNotificationKind, Payload: encoded})
	if err == nil || sender.calls != 1 {
		t.Fatalf("HandleOutbox() = %v, calls %d", err, sender.calls)
	}
}
