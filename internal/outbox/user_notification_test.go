package outbox

import (
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestUserNotificationPayloadRoundTrip(t *testing.T) {
	t.Parallel()
	payload := UserNotification{EventKey: "event-1", UserID: "user-1", ChatID: 42, Locale: "zh-CN",
		Kind: UserEventExpiration, OccurredAt: "2026-08-20T00:00:00Z", Facts: map[string]string{"combo": "Pro"}}
	encoded, err := EncodeUserNotification(payload)
	if err != nil {
		t.Fatalf("EncodeUserNotification(): %v", err)
	}
	decoded, err := DecodeUserNotification(model.OutboxJob{Kind: UserNotificationKind, Payload: encoded})
	if err != nil {
		t.Fatalf("DecodeUserNotification(): %v", err)
	}
	if decoded.EventKey != payload.EventKey || decoded.Facts["combo"] != "Pro" {
		t.Fatalf("decoded payload = %+v", decoded)
	}
}
