package notifications

import (
	"errors"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func newEventFields(kind string, copy copySet, facts map[string]string, location *time.Location) ([]cardField, error) {
	switch kind {
	case jobpayload.UserEventPurchaseQueued:
		fields, err := requiredFields(copy, facts, pair("combo", FactCombo), moneyPair("charged", FactCharge),
			datePair("starts", FactValidFrom, location), datePair("validUntil", FactValidUntil, location), moneyPair("balance", FactBalance))
		if err != nil {
			return nil, err
		}
		return insertOptional(fields, 2, copy, facts, "addOns", FactAddOns), nil
	case jobpayload.UserEventRenewalScheduled:
		return requiredFields(copy, facts, pair("combo", FactCombo), pair("termCount", FactTermCount),
			moneyPair("charged", FactCharge), datePair("starts", FactValidFrom, location),
			datePair("validUntil", FactValidUntil, location), moneyPair("balance", FactBalance))
	case jobpayload.UserEventAddonActivated:
		return requiredFields(copy, facts, pair("combo", FactCombo), pair("addedSquads", FactAddOns),
			moneyPair("charged", FactCharge), moneyPair("balance", FactBalance),
			datePair("validUntil", FactValidUntil, location), datePair("time", FactTime, location))
	case jobpayload.UserEventQueuedCancellation:
		return requiredFields(copy, facts, pair("combo", FactCombo), moneyPair("refunded", FactAmount),
			moneyPair("balance", FactBalance), datePair("time", FactTime, location))
	case jobpayload.UserEventAutoRenewalFailed:
		return requiredFields(copy, facts, pair("combo", FactCombo), datePair("expired", FactExpired, location),
			localizedReasonPair(copy, "RENEWAL_UNAVAILABLE"), fixedPair("noCharge", "notCharged", copy))
	case jobpayload.UserEventManualResetCompleted:
		return requiredFields(copy, facts, pair("combo", FactCombo), moneyPair("charged", FactCharge),
			moneyPair("balance", FactBalance), datePair("time", FactTime, location))
	case jobpayload.UserEventManualResetRefunded:
		return requiredFields(copy, facts, pair("combo", FactCombo), moneyPair("refunded", FactAmount),
			moneyPair("balance", FactBalance), localizedReasonPair(copy, "resetFailed"), datePair("time", FactTime, location))
	case jobpayload.UserEventMemberRefundCompleted:
		fields, err := requiredFields(copy, facts, pair("combo", FactCombo), moneyPair("refunded", FactAmount),
			moneyPair("balance", FactBalance), datePair("time", FactTime, location))
		if err != nil {
			return nil, err
		}
		return insertOptional(fields, 3, copy, facts, "replacement", FactReplacement), nil
	case jobpayload.UserEventMemberRefundFailed:
		return requiredFields(copy, facts, pair("combo", FactCombo), localizedReasonPair(copy, "memberRefundFailed"),
			datePair("time", FactTime, location))
	case jobpayload.UserEventProvisionConflict:
		return requiredFields(copy, facts, pair("cancelledCombos", FactCancelledCombos), moneyPair("refunded", FactAmount),
			moneyPair("balance", FactBalance), datePair("time", FactTime, location))
	default:
		return nil, errors.New("unsupported notification event")
	}
}

func localizedReasonPair(copy copySet, fallback string) fieldSpec {
	return fieldSpec{label: "reason", key: FactReason, format: func(value string) (string, error) {
		if translated := copy.values[value]; translated != "" {
			return translated, nil
		}
		return copy.values[fallback], nil
	}}
}
