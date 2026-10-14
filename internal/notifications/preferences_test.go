package notifications

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/preferences"
	"testing"
	"time"
)

type preferenceStub struct {
	value preferences.Preferences
	err   error
}

func (s *preferenceStub) UserPreferences(context.Context, string) (preferences.Preferences, error) {
	return s.value, s.err
}

func TestNotificationPreferencesAppliedAtDelivery(t *testing.T) {
	payload := notificationFixture(jobpayload.UserEventExpiration, "en", map[string]string{FactCombo: "Pro", FactExpired: "2026-10-07T00:00:00Z"})
	encoded, err := jobpayload.EncodeUserNotification(payload)
	if err != nil {
		t.Fatal(err)
	}
	reader := &preferenceStub{value: preferences.Defaults()}
	sender := &senderStub{}
	w := NewWorker(sender, nil, time.UTC, reader)
	reader.value.Notifications.Combos = false
	job := model.OutboxJob{Kind: jobpayload.UserNotificationKind, Payload: encoded}
	if err := w.HandleOutbox(context.Background(), job); err != nil || sender.calls != 0 {
		t.Fatalf("suppression: %v calls=%d", err, sender.calls)
	}
	reader.err = errors.New("database unavailable")
	if err := w.HandleOutbox(context.Background(), job); err == nil || sender.calls != 0 {
		t.Fatalf("lookup retry: %v calls=%d", err, sender.calls)
	}
	reader.err = nil
	reader.value.Notifications.Combos = true
	if err := w.HandleOutbox(context.Background(), job); err != nil || sender.calls != 1 {
		t.Fatalf("delivery: %v calls=%d", err, sender.calls)
	}
}
