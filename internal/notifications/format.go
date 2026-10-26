package notifications

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegramformat"
)

type cardField struct{ label, value string }

// Format renders one validated notification as bounded MarkdownV2.
func Format(payload jobpayload.UserNotification, location *time.Location) (string, error) {
	copy := copyFor(payload.Locale)
	title := copy.titles[payload.Kind]
	if payload.Kind == jobpayload.UserEventAdminUpdate {
		if specific := copy.titles["admin_"+payload.Facts[FactChange]]; specific != "" {
			title = specific
		}
	}
	if title == "" {
		return "", errors.New("notification title is unavailable")
	}
	fields, err := notificationFields(payload, copy, location)
	if err != nil {
		return "", err
	}
	parts := strings.SplitN(title, " ", 2)
	if len(parts) != 2 {
		return "", errors.New("notification title has no emoji separator")
	}
	lines := []string{parts[0] + " *" + telegramformat.Escape(parts[1]) + "*"}
	for _, field := range fields {
		lines = append(lines, "*"+telegramformat.Escape(field.label)+":* "+telegramformat.Escape(field.value))
	}
	guidance := copy.guidance[payload.Kind]
	if payload.Kind == jobpayload.UserEventProvisionConflict {
		guidance = "Choose a new username in VanSail Carpool to restore access."
		if payload.Locale == "zh-CN" {
			guidance = "请在 VanSail Carpool 中选择新用户名，重新开通套餐。"
		}
	}
	if guidance != "" {
		lines = append(lines, "", telegramformat.Escape(guidance))
	}
	return telegramformat.Limit(strings.Join(lines, "\n")), nil
}

func notificationFields(payload jobpayload.UserNotification, copy copySet, location *time.Location) ([]cardField, error) {
	facts := payload.Facts
	switch payload.Kind {
	case jobpayload.UserEventExpiration:
		return requiredFields(copy, facts, pair("combo", FactCombo), datePair("expired", FactExpired, location))
	case jobpayload.UserEventExpiryReminder:
		return requiredFields(copy, facts, pair("combo", FactCombo), datePair("expires", FactExpires, location),
			fixedPair("autoRenewal", "off", copy), fixedPair("queuedCombo", "none", copy))
	case jobpayload.UserEventQueuedActivation:
		return activationFields(copy, facts, location, false)
	case jobpayload.UserEventPurchaseActivation:
		return activationFields(copy, facts, location, true)
	case jobpayload.UserEventAutoRenewal:
		return renewalFields(copy, facts, location)
	case jobpayload.UserEventPaymentCredited:
		return requiredFields(copy, facts, translatedPair("provider", FactProvider, copy), pair("paymentAmount", FactPaymentAmount),
			moneyPair("txbCredited", FactAmount), moneyPair("balance", FactBalance), datePair("time", FactTime, location))
	case jobpayload.UserEventPaymentRefunded:
		fields, err := requiredFields(copy, facts, translatedPair("provider", FactProvider, copy), moneyPair("txbReversed", FactAmount),
			moneyPair("balance", FactBalance), datePair("time", FactTime, location))
		if err != nil {
			return nil, err
		}
		return insertOptional(fields, len(fields)-1, copy, facts, "cancelledCombos", FactCancelledCombos), nil
	case jobpayload.UserEventTrafficThreshold:
		return trafficFields(copy, facts)
	case jobpayload.UserEventAutomaticReset:
		return automaticResetFields(copy, facts, location)
	case jobpayload.UserEventAutomaticResetInsufficient:
		return automaticResetInsufficientFields(copy, facts)
	case jobpayload.UserEventAutomaticResetFailed:
		return requiredFields(copy, facts, pair("combo", FactCombo), moneyPair("refunded", FactAmount),
			moneyPair("balance", FactBalance), resetFailurePair(copy), datePair("time", FactTime, location))
	case jobpayload.UserEventGroupReward:
		return requiredFields(copy, facts, pair("messages", FactMessages), moneyPair("reward", FactReward),
			moneyPair("balance", FactBalance), datePair("time", FactTime, location))
	case jobpayload.UserEventAdminExtension, jobpayload.UserEventAdminUpdate:
		return adminFields(copy, facts, location, payload.Kind == jobpayload.UserEventAdminExtension, payload.Locale == "zh-CN")
	case jobpayload.UserEventNodeCompensation:
		return compensationFields(copy, facts, location, payload.Locale == "zh-CN")
	case jobpayload.UserEventPurchaseQueued, jobpayload.UserEventRenewalScheduled,
		jobpayload.UserEventAddonActivated, jobpayload.UserEventQueuedCancellation,
		jobpayload.UserEventAutoRenewalFailed, jobpayload.UserEventManualResetCompleted,
		jobpayload.UserEventManualResetRefunded, jobpayload.UserEventMemberRefundCompleted,
		jobpayload.UserEventMemberRefundFailed, jobpayload.UserEventProvisionConflict:
		return newEventFields(payload.Kind, copy, facts, location)
	default:
		return nil, errors.New("unsupported notification format")
	}
}

func automaticResetFields(copy copySet, facts map[string]string, location *time.Location) ([]cardField, error) {
	usage, err := trafficUsage(facts)
	if err != nil {
		return nil, err
	}
	return requiredFields(copy, facts, pair("combo", FactCombo), literalPair("traffic", usage),
		translatedPair("reset", FactReset, copy), moneyPair("charged", FactCharge), moneyPair("balance", FactBalance),
		datePair("time", FactTime, location))
}

func automaticResetInsufficientFields(copy copySet, facts map[string]string) ([]cardField, error) {
	usage, err := trafficUsage(facts)
	if err != nil {
		return nil, err
	}
	return requiredFields(copy, facts, pair("combo", FactCombo), literalPair("traffic", usage),
		moneyPair("requiredCharge", FactCharge), moneyPair("currentBalance", FactBalance),
		fixedPair("automation", "disabled", copy))
}

func activationFields(copy copySet, facts map[string]string, location *time.Location, purchased bool) ([]cardField, error) {
	specs := []fieldSpec{pair("combo", FactCombo)}
	if purchased {
		specs = append(specs, moneyPair("charged", FactCharge))
	}
	specs = append(specs, bytesPair("traffic", FactTrafficLimit), translatedPair("reset", FactReset, copy),
		datePair("validUntil", FactValidUntil, location))
	fields, err := requiredFields(copy, facts, specs...)
	if err != nil {
		return nil, err
	}
	return insertOptional(fields, len(fields)-1, copy, facts, "addOns", FactAddOns), nil
}

func renewalFields(copy copySet, facts map[string]string, location *time.Location) ([]cardField, error) {
	return requiredFields(copy, facts, pair("combo", FactCombo), debitPair("renewalDebit", FactRenewalDebit), moneyPair("balance", FactBalance), datePair("validUntil", FactValidUntil, location))
}

func trafficFields(copy copySet, facts map[string]string) ([]cardField, error) {
	usage, err := trafficUsage(facts)
	if err != nil {
		return nil, err
	}
	return requiredFields(copy, facts, pair("combo", FactCombo), literalPair("traffic", usage),
		bytesPair("remaining", FactRemaining), translatedPair("reset", FactReset, copy))
}

func trafficUsage(facts map[string]string) (string, error) {
	usage, err := byteRatio(facts, FactUsed, FactTrafficLimit)
	if err != nil {
		return "", err
	}
	used, _ := strconv.ParseInt(facts[FactUsed], 10, 64)
	limit, _ := strconv.ParseInt(facts[FactTrafficLimit], 10, 64)
	return usage + fmt.Sprintf(" (%.1f%%)", float64(used)*100/float64(limit)), nil
}
