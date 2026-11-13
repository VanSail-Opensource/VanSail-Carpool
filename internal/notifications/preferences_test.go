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

func TestHourlyFundingNotificationsUseComboDeliveryPreference(t *testing.T) {
	for _, kind := range []string{jobpayload.UserEventHourlyFundingWarning, jobpayload.UserEventHourlyFundingSuspended} {
		t.Run(kind, func(t *testing.T) {
			for _, enabled := range []bool{false, true} {
				payload := notificationFixture(kind, "en", map[string]string{
					FactSquad: "Hourly squad", FactHourlyPrice: "100", FactBalance: "50",
					FactTime: "2026-11-12T00:00:00Z",
				})
				encoded, err := jobpayload.EncodeUserNotification(payload)
				if err != nil {
					t.Fatal(err)
				}
				reader := &preferenceStub{value: preferences.Defaults()}
				reader.value.Notifications.Combos = enabled
				sender := &senderStub{}
				worker := NewWorker(sender, nil, time.UTC, reader)
				err = worker.HandleOutbox(context.Background(), model.OutboxJob{Kind: jobpayload.UserNotificationKind, Payload: encoded})
				want := 0
				if enabled {
					want = 1
				}
				if err != nil || sender.calls != want {
					t.Fatalf("combos enabled=%t: delivery error=%v calls=%d, want=%d", enabled, err, sender.calls, want)
				}
			}
		})
	}
}
