package telegrampm

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/telegram"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegramformat"
)

// ProfileFacts contains current, non-persisted account projections for a PM card.
type ProfileFacts struct {
	OnboardingState string
	Balance         model.Money
	PAYGTraffic     string
	PAYGSpent       model.Money
	ActiveCombo     *model.Purchase
	SquadNames      []string
	Rollover        *model.RolloverProjection
	UsedBytes       *int64
	AllocatedBytes  *int64
}

// Profile formats the current account facts and topic controls as MarkdownV2.
func Profile(item model.PMConversation, facts ProfileFacts, publicBaseURL, timezone string) telegram.TopicProfileRequest {
	name := strings.TrimSpace(item.FirstName + " " + item.LastName)
	if item.Username != "" {
		name += " (@" + item.Username + ")"
	}
	if name == "" {
		name = strconv.FormatInt(item.TelegramID, 10)
	}
	blockAction, muteAction, blockLabel, muteLabel := "block", "mute", "Block", "Mute"
	state := "Messages enabled"
	if item.Blocked {
		blockAction, blockLabel, state = "unblock", "Unblock", "Messages blocked"
	}
	if item.Muted {
		muteAction, muteLabel = "unmute", "Unmute"
		state += " · Silent relay"
	}
	if chinese(item.Locale) {
		blockLabel, muteLabel, state = "\u5c4f\u853d", "\u9759\u97f3", "\u5141\u8bb8\u79c1\u4fe1"
		if item.Blocked {
			blockLabel, state = "\u53d6\u6d88\u5c4f\u853d", "\u5df2\u5c4f\u853d\u79c1\u4fe1"
		}
		if item.Muted {
			muteLabel = "\u53d6\u6d88\u9759\u97f3"
			state += " · \u9759\u9ed8\u8f6c\u53d1"
		}
	}
	lines := []string{"*" + telegramformat.Escape(name) + "*", profileField("\U0001F194", "Telegram ID", strconv.FormatInt(item.TelegramID, 10))}
	if facts.OnboardingState != "complete" {
		lines = append(lines, profileField("\u26A0\uFE0F", "Status", localized(item.Locale, "Not yet onboarded", "\u5c1a\u672a\u5b8c\u6210\u6ce8\u518c")))
	} else {
		balance := facts.Balance.Display
		if balance == "" {
			balance = model.TXBMoney(0).Display
		}
		lines = append(lines, profileField("\U0001F4B0", localized(item.Locale, "Balance", "\u4f59\u989d"), balance))
		paygSpent := facts.PAYGSpent.Display
		if paygSpent == "" {
			paygSpent = model.TXBMoney(0).Display
		}
		lines = append(lines, profileField("\U0001F4CA", localized(item.Locale, "PAYG (72h)", "PAYG（72小时）"), formatProfileDecimalBytes(facts.PAYGTraffic)+" / "+paygSpent))
		if purchase := facts.ActiveCombo; purchase != nil {
			lines = append(lines,
				profileField("\U0001F4E6", localized(item.Locale, "Active combo", "\u5f53\u524d\u5957\u9910"), purchase.ComboName),
				profileField("\U0001F5D3\uFE0F", localized(item.Locale, "Term", "\u6709\u6548\u671f"), profileDate(purchase.ValidFrom, timezone)+" \u2013 "+profileDate(purchase.ValidUntil, timezone)),
				profileField("\U0001F504", localized(item.Locale, "Auto renew", "\u81ea\u52a8\u7eed\u8d39"), localizedBool(item.Locale, purchase.AutoRenewEnabled)),
			)
			if len(facts.SquadNames) > 0 {
				lines = append(lines, profileField("\U0001F6E3\uFE0F", localized(item.Locale, "Optional squads", "\u53ef\u9009\u7ebf\u8def"), strings.Join(facts.SquadNames, ", ")))
			}
			if purchase.CouponGrantID != nil {
				lines = append(lines, profileField("\U0001F39F\uFE0F", localized(item.Locale, "Coupon discount", "\u4f18\u60e0\u5238\u6298\u6263"), purchase.CouponDiscount.Display))
			}
			if facts.UsedBytes != nil && facts.AllocatedBytes != nil && *facts.UsedBytes >= 0 && *facts.AllocatedBytes > 0 {
				traffic := formatProfileBytes(*facts.UsedBytes) + " / " + formatProfileBytes(*facts.AllocatedBytes)
				lines = append(lines, profileField("\U0001F4CA", localized(item.Locale, "Used / total traffic", "\u5df2\u7528 / \u603b\u6d41\u91cf"), traffic))
			}
			if !purchase.AutoRenewEnabled {
				lines = append(lines, profileField("\u267B\uFE0F", localized(item.Locale, "Rollover", "\u7ed3\u8f6c\u9884\u6d4b"), localized(item.Locale, "Disabled", "\u672a\u542f\u7528")))
			} else if facts.Rollover != nil {
				value := localized(item.Locale, "Temporarily unavailable", "\u6682\u4e0d\u53ef\u7528")
				if facts.Rollover.PredictedRollover != nil {
					value = facts.Rollover.PredictedRollover.Display
				}
				lines = append(lines, profileField("\u267B\uFE0F", localized(item.Locale, "Rollover prediction", "\u7ed3\u8f6c\u9884\u6d4b"), value))
			}
		}
		lines = append(lines, profileField("\u2699\uFE0F", localized(item.Locale, "PM status", "\u79c1\u4fe1\u72b6\u6001"), state))
	}
	refreshLabel, profileLabel := "Refresh", "Open profile"
	if chinese(item.Locale) {
		refreshLabel, profileLabel = "\u5237\u65b0\u8d44\u6599", "\u6253\u5f00\u7528\u6237\u8d44\u6599"
	}
	buttons := [][]telegram.InlineKeyboardButton{
		{{Text: blockLabel, CallbackData: "pm:" + blockAction + ":" + item.ID}, {Text: muteLabel, CallbackData: "pm:" + muteAction + ":" + item.ID}},
		{{Text: refreshLabel, CallbackData: "pm:refresh:" + item.ID}},
	}
	if profileURL := browserProfileURL(publicBaseURL, item.UserID); profileURL != "" {
		buttons = append(buttons, []telegram.InlineKeyboardButton{{Text: profileLabel, URL: profileURL}})
	}
	return telegram.TopicProfileRequest{ChatID: item.ChatID, MessageThreadID: item.TopicID, MessageID: item.ProfileMessageID,
		Text: telegramformat.Limit(strings.Join(lines, "\n")), ReplyMarkup: telegram.InlineKeyboardMarkup{InlineKeyboard: buttons}}
}

func profileField(emoji, label, value string) string {
	return emoji + " *" + telegramformat.Escape(label) + ":* " + telegramformat.Escape(value)
}

func localized(locale, english, chineseText string) string {
	if chinese(locale) {
		return chineseText
	}
	return english
}

func localizedBool(locale string, enabled bool) string {
	if enabled {
		return localized(locale, "Enabled", "\u5df2\u542f\u7528")
	}
	return localized(locale, "Disabled", "\u672a\u542f\u7528")
}

func profileDate(value time.Time, timezone string) string {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		location = time.UTC
	}
	return value.In(location).Format(time.DateOnly)
}

func formatProfileBytes(value int64) string {
	return formatProfileByteAmount(float64(value))
}

func formatProfileDecimalBytes(value string) string {
	amount, err := strconv.ParseFloat(value, 64)
	if err != nil || amount < 0 {
		amount = 0
	}
	return formatProfileByteAmount(amount)
}

func formatProfileByteAmount(amount float64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	unit := 0
	for amount >= 1024 && unit < len(units)-1 {
		amount /= 1024
		unit++
	}
	return fmt.Sprintf("%.2f %s", amount, units[unit])
}

func browserProfileURL(publicBaseURL, userID string) string {
	base, err := url.Parse(strings.TrimSpace(publicBaseURL))
	if err != nil || (base.Scheme != "https" && base.Scheme != "http") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || userID == "" {
		return ""
	}
	for _, char := range userID {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_') {
			return ""
		}
	}
	return base.ResolveReference(&url.URL{Path: "/admin/users/" + userID}).String()
}
