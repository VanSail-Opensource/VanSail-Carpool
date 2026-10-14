package outbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// PaymentSuccessAnnouncement is the immutable payment snapshot carried by the
// durable Telegram announcement job.
type PaymentSuccessAnnouncement struct {
	OrderID         string `json:"orderId"`
	Provider        string `json:"provider"`
	ProviderName    string `json:"providerName,omitempty"`
	Channel         string `json:"channel"`
	TXBMinor        int64  `json:"txbMinor"`
	PayableAmount   string `json:"payableAmount"`
	PayableCurrency string `json:"payableCurrency"`
	Username        string `json:"username"`
}

type AffiliateSuccess struct {
	SettlementID    string `json:"settlementId"`
	ChatID          int64  `json:"chatId"`
	Locale          string `json:"locale"`
	InviteeName     string `json:"inviteeName"`
	SettledAt       string `json:"settledAt"`
	CommissionMinor int64  `json:"commissionMinor"`
	TierName        string `json:"tierName"`
}

type AffiliateTierUpgrade struct {
	AwardID           string `json:"awardId"`
	ChatID            int64  `json:"chatId"`
	Locale            string `json:"locale"`
	TierName          string `json:"tierName"`
	RewardDescription string `json:"rewardDescription"`
	AwardedAt         string `json:"awardedAt"`
}

// TargetID extracts one required string identifier without constraining other fields.
func TargetID(job model.OutboxJob, field string) (string, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return "", fmt.Errorf("decode %s job payload: %w", job.Kind, err)
	}
	raw, exists := payload[field]
	if !exists {
		return "", errors.New("outbox payload is missing " + field)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("decode %s job target %s: %w", job.Kind, field, err)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("outbox payload is missing " + field)
	}
	return value, nil
}

// DecodePaymentSuccessAnnouncement validates one canonical announcement job.
func DecodePaymentSuccessAnnouncement(job model.OutboxJob) (PaymentSuccessAnnouncement, error) {
	if job.Kind != PaymentSuccessAnnouncementKind {
		return PaymentSuccessAnnouncement{}, errors.New("unexpected payment announcement job kind")
	}
	var payload PaymentSuccessAnnouncement
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return PaymentSuccessAnnouncement{}, fmt.Errorf("decode payment announcement payload: %w", err)
	}
	payload.OrderID = strings.TrimSpace(payload.OrderID)
	payload.Provider = strings.ToLower(strings.TrimSpace(payload.Provider))
	payload.ProviderName = strings.TrimSpace(payload.ProviderName)
	payload.Channel = strings.ToLower(strings.TrimSpace(payload.Channel))
	payload.PayableAmount = strings.TrimSpace(payload.PayableAmount)
	payload.PayableCurrency = strings.ToUpper(strings.TrimSpace(payload.PayableCurrency))
	payload.Username = strings.TrimSpace(payload.Username)
	if payload.OrderID == "" || payload.Provider == "" || payload.TXBMinor <= 0 ||
		payload.PayableAmount == "" || payload.PayableCurrency == "" || payload.Username == "" {
		return PaymentSuccessAnnouncement{}, errors.New("payment announcement payload is incomplete")
	}
	return payload, nil
}

func DecodeAffiliateSuccess(job model.OutboxJob) (AffiliateSuccess, error) {
	var payload AffiliateSuccess
	if job.Kind != AffiliateSuccessKind {
		return payload, errors.New("unexpected affiliate success job kind")
	}
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return payload, fmt.Errorf("decode affiliate success payload: %w", err)
	}
	payload.InviteeName, payload.TierName = strings.TrimSpace(payload.InviteeName), strings.TrimSpace(payload.TierName)
	if payload.SettlementID == "" || payload.ChatID <= 0 || payload.Locale == "" || payload.SettledAt == "" || payload.CommissionMinor < 0 || payload.TierName == "" {
		return payload, errors.New("affiliate success payload is incomplete")
	}
	return payload, nil
}

func DecodeAffiliateTierUpgrade(job model.OutboxJob) (AffiliateTierUpgrade, error) {
	var payload AffiliateTierUpgrade
	if job.Kind != AffiliateTierUpgradeKind {
		return payload, errors.New("unexpected affiliate upgrade job kind")
	}
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return payload, fmt.Errorf("decode affiliate upgrade payload: %w", err)
	}
	payload.TierName, payload.RewardDescription = strings.TrimSpace(payload.TierName), strings.TrimSpace(payload.RewardDescription)
	if payload.AwardID == "" || payload.ChatID <= 0 || payload.Locale == "" || payload.TierName == "" || payload.RewardDescription == "" || payload.AwardedAt == "" {
		return payload, errors.New("affiliate upgrade payload is incomplete")
	}
	return payload, nil
}
