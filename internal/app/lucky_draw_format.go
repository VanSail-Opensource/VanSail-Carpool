package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegramformat"
)

func drawEscape(value string) string { return telegramformat.Escape(value) }
func drawMention(user model.User) string {
	label := strings.TrimSpace(user.TelegramFirstName + " " + user.TelegramLastName)
	if user.TelegramUsername != "" {
		label = "@" + user.TelegramUsername
	}
	if label == "" {
		label = "Member"
	}
	return "[" + drawEscape(label) + "](tg://user?id=" + strconv.FormatInt(user.TelegramID, 10) + ")"
}
func drawProgress(seats, threshold int) string {
	filled := 0
	if threshold > 0 {
		filled = min(10, seats*10/threshold)
	}
	return strings.Repeat("🟩", filled) + strings.Repeat("⬛", 10-filled)
}
func drawAnnouncement(draw activity.LuckyDraw, seats int) string {
	var b strings.Builder
	probabilities := drawRaffleProbabilities(draw)
	b.WriteString("🎟 *" + drawEscape(draw.Name) + "*\n")
	if draw.Description != "" {
		b.WriteString(drawEscape(draw.Description) + "\n")
	}
	b.WriteString("\n💰 *Entry*  " + drawEscape(model.TXBMoney(draw.FeeMinor).Display) + "\n")
	b.WriteString("👥 *Seats*  " + drawEscape(fmt.Sprintf("%d / %d", seats, draw.Threshold)) + "\n")
	b.WriteString(drawProgress(seats, draw.Threshold) + "\n")
	b.WriteString("✉️ *Join*  " + drawEscape(draw.Keyword) + "  or  /" + drawEscape(draw.Command) + "\n")
	b.WriteString("\n🎁 *Prizes*\n")
	for index, prize := range draw.Prizes {
		b.WriteString("• " + drawEscape(prize.Name) + "  " + drawEscape(fmt.Sprintf("%.2f%%", float64(probabilities[index])/100)) + "\n")
	}
	return telegramformat.Limit(b.String())
}
func drawRaffleProbabilities(draw activity.LuckyDraw) []int {
	result := make([]int, len(draw.Prizes))
	if draw.Threshold <= 0 {
		return result
	}
	type remainder struct {
		index int
		value int64
	}
	remainders := make([]remainder, 0, len(draw.Prizes))
	total := 0
	for index, prize := range draw.Prizes {
		scaled := prize.Stock * 10000
		result[index] = int(scaled / int64(draw.Threshold))
		total += result[index]
		remainders = append(remainders, remainder{index, scaled % int64(draw.Threshold)})
	}
	sort.SliceStable(remainders, func(i, j int) bool { return remainders[i].value > remainders[j].value })
	for i := 0; i < 10000-total; i++ {
		result[remainders[i].index]++
	}
	return result
}
func drawRewardContent(reward activity.Reward) string {
	value := int64(0)
	if reward.ResolvedValue != nil {
		value = *reward.ResolvedValue
	}
	switch reward.Kind {
	case activity.RewardTXBDelta:
		if reward.ResolvedValue == nil {
			value = reward.TXBDeltaMinor
		}
		return model.TXBMoney(value).Display
	case activity.RewardBalanceMultiplier:
		return fmt.Sprintf("%.2fx balance", float64(value)/10000)
	case activity.RewardTrafficGrant:
		return fmt.Sprintf("%+d GiB traffic", value)
	case activity.RewardSubscriptionExtension:
		if reward.ResolvedValue == nil {
			value = int64(reward.ExtensionDays) * 24
		}
		return fmt.Sprintf("%d hours of subscription", value)
	case activity.RewardEntitlementGrant:
		return fmt.Sprintf("Custom combo, %s renewal, %d GiB traffic, %d squads",
			model.TXBMoney(reward.RenewalPriceMinor).Display, reward.TrafficLimitBytes/(1<<30), len(reward.SquadUUIDs))
	case activity.RewardSquadAccess:
		return fmt.Sprintf("Access to %d squads until this term ends", len(reward.SquadUUIDs))
	case activity.RewardCoreComboSwitch:
		return "Core combo " + reward.ComboID
	case activity.RewardTrafficReset:
		return "Traffic counter reset"
	case activity.RewardCouponRecurring, activity.RewardCouponOnce:
		if reward.DiscountMode == "percent" {
			return fmt.Sprintf("%.2f%% discount", float64(value)/100)
		}
		return model.TXBMoney(value).Display + " discount"
	default:
		return ""
	}
}
func drawInstantMessage(user model.User, result activity.DrawResult) string {
	return "🎉 Congratulations " + drawMention(user) + "\\!\n" +
		"🎁 *" + drawEscape(result.PrizeName) + "*\n" +
		drawEscape(drawRewardContent(result.Reward))
}
func drawPrivateMessage(result activity.DrawResult) string {
	message := "🎉 Congratulations\\!\n🎁 *" + drawEscape(result.PrizeName) + "*\n" + drawEscape(drawRewardContent(result.Reward))
	switch result.Reward.Kind {
	case activity.RewardEntitlementGrant, activity.RewardSquadAccess, activity.RewardCoreComboSwitch,
		activity.RewardTrafficGrant, activity.RewardTrafficReset, activity.RewardSubscriptionExtension:
		message += "\n⏳ Provider changes are being applied"
	}
	return message
}
