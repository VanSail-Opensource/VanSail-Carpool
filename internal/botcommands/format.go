package botcommands

import (
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const MessageLimit = 4096

type cardField struct {
	label string
	value string
}

// FormatUnknown renders a safe unknown-command response.
func FormatUnknown(copy Copy) string {
	return formatCard(copy.UnknownTitle, cardField{copy.StatusLabel, copy.Unknown})
}

// FormatUnavailable renders a temporary command-data failure.
func FormatUnavailable(copy Copy) string {
	return formatCard(copy.UnavailableTitle, cardField{copy.StatusLabel, copy.Unavailable})
}

// FormatGroupBoostRequired explains why an unboosted member cannot check in.
func FormatGroupBoostRequired(copy Copy) string {
	return formatCard(copy.SignInReward, cardField{copy.StatusLabel, copy.GroupBoostRequired})
}

// FormatGroupBoostUnavailable distinguishes provider failures from ineligibility.
func FormatGroupBoostUnavailable(copy Copy) string {
	return formatCard(copy.UnavailableTitle, cardField{copy.StatusLabel, copy.GroupBoostUnavailable})
}

// FormatNoSubscription renders an empty subscription state.
func FormatNoSubscription(copy Copy) string {
	return formatCard(copy.NoSubscriptionTitle, cardField{copy.StatusLabel, copy.NoSubscription})
}

// FormatDeductUsage renders safe administrator command usage guidance.
func FormatDeductUsage(copy Copy) string {
	return formatCard(copy.DeductTitle, cardField{copy.StatusLabel, copy.DeductUsage})
}

// FormatDeductRejected renders a deduction rejection without exposing details.
func FormatDeductRejected(copy Copy) string {
	return formatCard(copy.DeductTitle, cardField{copy.StatusLabel, copy.DeductRejected})
}

// FormatDeductSucceeded renders a successful balance deduction.
func FormatDeductSucceeded(copy Copy, amount model.Money) string {
	return formatCard(copy.DeductTitle,
		cardField{copy.StatusLabel, copy.DeductSucceeded},
		cardField{copy.AmountLabel, amount.Display},
	)
}

func formatCard(title string, fields ...cardField) string {
	return formatCardWithItems(title, fields, "", nil)
}

func formatCardWithItems(title string, fields []cardField, itemLabel string, items []string) string {
	lines := []string{"*" + escapeMarkdownV2(title) + "*"}
	for _, field := range fields {
		lines = append(lines, "*"+escapeMarkdownV2(field.label)+":* "+escapeMarkdownV2(field.value))
	}
	if len(items) > 0 {
		lines = append(lines, "", "*"+escapeMarkdownV2(itemLabel)+":*")
		for _, item := range items {
			lines = append(lines, escapeMarkdownV2(item))
		}
	}
	return Limit(strings.Join(lines, "\n"))
}

// Limit keeps MarkdownV2 replies below Telegram's 4096-code-point limit.
func Limit(value string) string {
	return limitMarkdownV2(value)
}
