package botcommands

import (
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// RolloverState identifies the display outcome of the authoritative projection.
type RolloverState string

const (
	RolloverPredicted   RolloverState = "predicted"
	RolloverIneligible  RolloverState = "ineligible"
	RolloverDisabled    RolloverState = "disabled"
	RolloverUnavailable RolloverState = "unavailable"
)

// RolloverSummary carries localized state and an optional authoritative value.
type RolloverSummary struct {
	State  RolloverState
	Amount *model.Money
}

// FormatSubscription renders current usage and a top-node distribution.
func FormatSubscription(copy Copy, dashboard model.Dashboard, _ time.Time) string {
	if dashboard.ActivePurchase == nil || dashboard.Statistics == nil {
		return FormatNoSubscription(copy)
	}
	used := parseBytes(dashboard.Statistics.UsedTrafficBytes)
	limit := parseBytes(dashboard.Statistics.TrafficLimitBytes)
	percent := percentage(used, limit)
	lines := []string{
		"📊 *" + escapeMarkdownV2(copy.SubscriptionTitle) + "*",
		escapeMarkdownV2(fmt.Sprintf("%s %d%% | %s/%s", usageBar(percent), int(percent+0.5), formatBytes(used), formatBytes(limit))),
	}
	nodes, total := displayNodes(dashboard.Statistics.TopNodes)
	if total > 0 {
		lines = append(lines, "", escapeMarkdownV2("["+nodeDistribution(nodes, total)+"] "+formatBytes(total)), "")
		for index, node := range nodes {
			line := fmt.Sprintf("%s %s (%s) - %s (%.1f%%)", nodeMarkers[index], node.Name,
				strings.ToUpper(node.CountryCode), formatBytes(node.Bytes), share(node.Bytes, total))
			lines = append(lines, escapeMarkdownV2(line))
		}
	}
	return Limit(strings.Join(lines, "\n"))
}

// FormatCombo renders the active purchase with localized rollover state.
func FormatCombo(copy Copy, purchase *model.Purchase, squadNames []string, rollover RolloverSummary) string {
	if purchase == nil {
		return FormatNoSubscription(copy)
	}
	if len(squadNames) == 0 {
		squadNames = append([]string(nil), purchase.SquadUUIDs...)
	}
	lines := []string{
		"🎫 *" + escapeMarkdownV2(copy.ComboTitle) + "*",
		formatEmojiField("🚘", copy.ComboLabel, purchase.ComboName),
		formatEmojiField("🛣️", copy.SquadsLabel, strings.Join(squadNames, ", ")),
		formatEmojiField("📦", copy.TrafficLabel, formatBytes(purchase.TrafficLimitBytes)),
		formatEmojiField("🔄", copy.ResetLabel, resetStrategyText(copy, purchase.ResetStrategy)),
		formatEmojiField("♻️", copy.RolloverLabel, rolloverText(copy, rollover)),
	}
	return Limit(strings.Join(lines, "\n"))
}

func resetStrategyText(copy Copy, strategy string) string {
	switch strings.ToUpper(strings.TrimSpace(strategy)) {
	case "DAY":
		return copy.ResetDaily
	case "WEEK":
		return copy.ResetWeekly
	case "MONTH_ROLLING":
		return copy.ResetMonthly
	default:
		return strategy
	}
}

func formatEmojiField(icon, label, value string) string {
	return icon + " *" + escapeMarkdownV2(label) + ":* " + escapeMarkdownV2(value)
}

func rolloverText(copy Copy, summary RolloverSummary) string {
	switch summary.State {
	case RolloverPredicted:
		if summary.Amount != nil {
			return copy.RolloverWill + " " + summary.Amount.Display
		}
		return copy.RolloverUnavailable
	case RolloverIneligible:
		return copy.RolloverWillNot
	case RolloverDisabled:
		return copy.RolloverCannot
	default:
		return copy.RolloverUnavailable
	}
}
