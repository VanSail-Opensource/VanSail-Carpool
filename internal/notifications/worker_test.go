package notifications

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

type senderStub struct {
	calls int
	err   error
}

func TestWorkerSeparatesRecipientFailuresRateLimitsAndProviderFailures(t *testing.T) {
	payload := notificationFixture(jobpayload.UserEventExpiration, "en", map[string]string{FactCombo: "Pro", FactExpired: "2026-08-20T00:00:00Z"})
	encoded, err := jobpayload.EncodeUserNotification(payload)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name           string
		failure        *telegram.APIError
		terminal, wait bool
	}{
		{"blocked", &telegram.APIError{ErrorCode: 403, Description: "Forbidden: bot was blocked by the user"}, true, false},
		{"not found", &telegram.APIError{ErrorCode: 400, Description: "Bad Request: chat not found"}, true, false},
		{"rate limited", &telegram.APIError{ErrorCode: 429, RetryAfter: 120}, false, true},
		{"gateway", &telegram.APIError{ErrorCode: 502}, false, false},
		{"unknown forbidden", &telegram.APIError{ErrorCode: 403, Description: "Forbidden: unknown policy"}, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			worker := NewWorker(&senderStub{err: test.failure}, nil, time.UTC)
			err := worker.HandleOutbox(context.Background(), model.OutboxJob{Kind: jobpayload.UserNotificationKind, Payload: encoded})
			var terminal *outbox.TerminalError
			var wait *outbox.WaitError
			if errors.As(err, &terminal) != test.terminal || errors.As(err, &wait) != test.wait {
				t.Fatal(err)
			}
			if test.wait && wait.RetryDelay() != 120*time.Second {
				t.Fatal("provider retry deadline ignored", wait)
			}
		})
	}
}

func (s *senderStub) SendMarkdownV2Message(context.Context, int64, int64, string) error {
	s.calls++
	return s.err
}

func TestWorkerAcknowledgesRetiredOutageNotificationWithoutDelivery(t *testing.T) {
	sender := &senderStub{}
	worker := NewWorker(sender, nil, time.UTC)
	err := worker.HandleOutbox(context.Background(), model.OutboxJob{Kind: jobpayload.UserNotificationKind, Payload: `{"kind":"node_compensation","eventKey":"legacy-approved"}`})
	if err != nil || sender.calls != 0 {
		t.Fatal("retired notification was delivered or retried", sender.calls, err)
	}
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
