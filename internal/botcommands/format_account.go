package botcommands

import (
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// FormatBalance renders a self-only TXB balance reply without a title.
func FormatBalance(copy Copy, money model.Money) string {
	return Limit(escapeMarkdownV2(copy.BalanceLabel + ": " + money.Display))
}

// FormatCheckIn renders an idempotent daily check-in result against an optional
// cached historical average in TXB minor units.
func FormatCheckIn(copy Copy, result activity.DailyCheckIn, averageMinor *int64) string {
	return FormatCheckInWithMoney(copy, result, averageMinor, model.TXBMoney(result.RewardMinor), model.TXBMoney(result.BalanceAfterMinor))
}

// FormatCheckInWithMoney renders a check-in result with member-selected display values.
func FormatCheckInWithMoney(copy Copy, result activity.DailyCheckIn, averageMinor *int64, reward, balance model.Money) string {
	tone := copy.SignInNeutral
	if result.AlreadyClaimed {
		tone = copy.SignInAlready
	} else if averageMinor != nil {
		switch {
		case result.RewardMinor > *averageMinor:
			tone = copy.SignInAbove
		case result.RewardMinor < *averageMinor:
			tone = copy.SignInBelow
		default:
			tone = copy.SignInEqual
		}
	}
	lines := []string{
		escapeMarkdownV2(tone),
		"🎁 *" + escapeMarkdownV2(copy.SignInReward) + ":* " + escapeMarkdownV2("+"+reward.Display),
		"💰 *" + escapeMarkdownV2(copy.BalanceLabel) + ":* " + escapeMarkdownV2(balance.Display),
	}
	return Limit(strings.Join(lines, "\n"))
}

// FormatStart renders the bot entry-point sentence without a title.
func FormatStart(copy Copy) string {
	return Limit(escapeMarkdownV2(copy.Start))
}

// FormatWelcome renders a safe MarkdownV2 text mention for a joining member.
func FormatWelcome(copy Copy, telegramID int64, displayName string) string {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = copy.MemberLabel
	}
	return Limit(escapeMarkdownV2(copy.WelcomePrefix) + safeMention(telegramID, displayName) + escapeMarkdownV2(copy.WelcomeSuffix))
}
