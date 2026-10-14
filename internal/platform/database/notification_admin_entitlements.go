package database

import (
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func adminEntitlementEditNotification(before model.Purchase, input AdminEntitlementEditInput, newCombo string,
	now time.Time) (string, map[string]string) {
	kind := jobpayload.UserEventAdminUpdate
	facts := adminNotificationBase("entitlement_edit", input.Reason, now)
	if input.ValidUntil.After(before.ValidUntil) {
		kind = jobpayload.UserEventAdminExtension
		facts[notifications.FactAddedSeconds] = strconv.FormatInt(int64(input.ValidUntil.Sub(before.ValidUntil)/time.Second), 10)
		facts[notifications.FactPreviousExpiry] = before.ValidUntil.Format(time.RFC3339Nano)
		facts[notifications.FactNewExpiry] = input.ValidUntil.Format(time.RFC3339Nano)
	} else {
		addChangedTime(facts, before.ValidUntil, input.ValidUntil, notifications.FactPreviousExpiry, notifications.FactNewExpiry)
	}
	if before.ComboID != input.ComboID {
		facts[notifications.FactPreviousCombo], facts[notifications.FactNewCombo] = before.ComboName, newCombo
	}
	addChangedTime(facts, before.ValidFrom, input.ValidFrom, notifications.FactPreviousValidFrom, notifications.FactNewValidFrom)
	addChanged(facts, before.Status, input.Status, notifications.FactPreviousStatus, notifications.FactNewStatus)
	addChangedInt(facts, before.TrafficLimitBytes, input.TrafficLimitBytes, notifications.FactPreviousTraffic, notifications.FactNewTraffic)
	addChanged(facts, before.ResetStrategy, input.ResetStrategy, notifications.FactPreviousReset, notifications.FactNewReset)
	addChanged(facts, squadSummary(before.SquadUUIDs), squadSummary(input.SquadUUIDs),
		notifications.FactPreviousSquads, notifications.FactNewSquads)
	return kind, facts
}

func adminNotificationBase(change, reason string, now time.Time) map[string]string {
	return map[string]string{
		notifications.FactChange: change, notifications.FactReason: strings.TrimSpace(reason),
		notifications.FactTime: now.UTC().Format(time.RFC3339Nano),
	}
}

func addChanged(facts map[string]string, before, after, beforeKey, afterKey string) {
	if before != after {
		facts[beforeKey], facts[afterKey] = before, after
	}
}

func addChangedInt(facts map[string]string, before, after int64, beforeKey, afterKey string) {
	if before != after {
		facts[beforeKey], facts[afterKey] = strconv.FormatInt(before, 10), strconv.FormatInt(after, 10)
	}
}

func addChangedTime(facts map[string]string, before, after time.Time, beforeKey, afterKey string) {
	if !before.Equal(after) {
		facts[beforeKey], facts[afterKey] = before.UTC().Format(time.RFC3339Nano), after.UTC().Format(time.RFC3339Nano)
	}
}

func squadSummary(values []string) string {
	if len(values) == 0 {
		return "—"
	}
	const visible = 12
	displayed := values
	if len(displayed) > visible {
		displayed = displayed[:visible]
	}
	summary := strconv.Itoa(len(values)) + ": " + strings.Join(displayed, ", ")
	if hidden := len(values) - len(displayed); hidden > 0 {
		summary += ", … +" + strconv.Itoa(hidden)
	}
	return summary
}
