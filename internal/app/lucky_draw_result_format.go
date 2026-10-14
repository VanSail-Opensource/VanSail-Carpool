package app

import (
	"strings"
	"unicode/utf8"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func drawRaffleResultMessages(draw activity.LuckyDraw, results []activity.DrawResult, users map[string]model.User) []string {
	winners := make(map[string][]string)
	seen := make(map[string]bool)
	order := make([]string, 0)
	for _, result := range results {
		if !seen[result.UserID] {
			seen[result.UserID] = true
			order = append(order, result.UserID)
		}
		if result.Reward.Kind != activity.RewardNone {
			winners[result.UserID] = append(winners[result.UserID], drawEscape(result.PrizeName)+" · "+drawEscape(drawRewardContent(result.Reward)))
		}
	}
	lines := []string{"🎊 *" + drawEscape(draw.Name) + " results*", "🎁 *Winners*"}
	nonWinners := make([]string, 0)
	for _, userID := range order {
		mention := drawMention(users[userID])
		if len(winners[userID]) == 0 {
			nonWinners = append(nonWinners, "• "+mention)
			continue
		}
		lines = append(lines, "• "+mention)
		for _, reward := range winners[userID] {
			lines = append(lines, "  ↳ "+reward)
		}
	}
	if len(winners) == 0 {
		lines = append(lines, "• None")
	}
	lines = append(lines, "", "🍀 *No reward*")
	if len(nonWinners) == 0 {
		lines = append(lines, "• None")
	} else {
		lines = append(lines, nonWinners...)
	}
	lines = append(lines, "", "🎉 Congratulations to everyone who won\\!")
	return splitDrawMessage(lines)
}

func splitDrawMessage(lines []string) []string {
	messages := make([]string, 0, 1)
	var current strings.Builder
	for _, line := range lines {
		if current.Len() > 0 && utf8.RuneCountInString(current.String())+utf8.RuneCountInString(line)+1 > 3600 {
			messages = append(messages, current.String())
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteByte('\n')
		}
		current.WriteString(line)
	}
	if current.Len() > 0 {
		messages = append(messages, current.String())
	}
	return messages
}
