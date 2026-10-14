package outbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// UserNotification is an immutable private-chat notification snapshot.
type UserNotification struct {
	EventKey   string            `json:"eventKey"`
	UserID     string            `json:"userId"`
	ChatID     int64             `json:"chatId"`
	Locale     string            `json:"locale"`
	Kind       string            `json:"kind"`
	OccurredAt string            `json:"occurredAt"`
	Facts      map[string]string `json:"facts"`
}

// EncodeUserNotification validates and encodes one canonical payload.
func EncodeUserNotification(payload UserNotification) (string, error) {
	payload.EventKey = strings.TrimSpace(payload.EventKey)
	payload.UserID = strings.TrimSpace(payload.UserID)
	payload.Locale = strings.TrimSpace(payload.Locale)
	payload.Kind = strings.TrimSpace(payload.Kind)
	payload.OccurredAt = strings.TrimSpace(payload.OccurredAt)
	if err := validateUserNotification(payload); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode user notification: %w", err)
	}
	return string(encoded), nil
}

// DecodeUserNotification validates one durable Telegram job.
func DecodeUserNotification(job model.OutboxJob) (UserNotification, error) {
	var payload UserNotification
	if job.Kind != UserNotificationKind {
		return payload, errors.New("unexpected user notification job kind")
	}
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return payload, fmt.Errorf("decode user notification: %w", err)
	}
	if err := validateUserNotification(payload); err != nil {
		return payload, err
	}
	return payload, nil
}

func validateUserNotification(payload UserNotification) error {
	if payload.EventKey == "" || payload.UserID == "" || payload.ChatID <= 0 || payload.OccurredAt == "" || len(payload.Facts) == 0 {
		return errors.New("user notification payload is incomplete")
	}
	if payload.Locale != "en" && payload.Locale != "zh-CN" {
		return errors.New("user notification locale is unsupported")
	}
	switch payload.Kind {
	case UserEventExpiration, UserEventExpiryReminder, UserEventQueuedActivation, UserEventPurchaseActivation,
		UserEventAutoRenewal, UserEventPaymentCredited, UserEventPaymentRefunded,
		UserEventTrafficThreshold, UserEventAutomaticReset, UserEventAutomaticResetInsufficient,
		UserEventAutomaticResetFailed, UserEventGroupReward, UserEventAdminExtension, UserEventAdminUpdate,
		UserEventNodeCompensation, UserEventPurchaseQueued, UserEventRenewalScheduled,
		UserEventAddonActivated, UserEventQueuedCancellation, UserEventAutoRenewalFailed,
		UserEventManualResetCompleted, UserEventManualResetRefunded,
		UserEventMemberRefundCompleted, UserEventMemberRefundFailed, UserEventProvisionConflict:
		return nil
	default:
		return errors.New("user notification kind is unsupported")
	}
}
