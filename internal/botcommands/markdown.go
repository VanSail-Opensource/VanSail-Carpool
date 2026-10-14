package botcommands

import (
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegramformat"
)

const markdownV2Ellipsis = `\.\.\.`

func escapeMarkdownV2(value string) string {
	return telegramformat.Escape(value)
}

func safeMention(telegramID int64, label string) string {
	return "[" + escapeMarkdownV2(label) + "](tg://user?id=" + strconv.FormatInt(telegramID, 10) + ")"
}

func limitMarkdownV2(value string) string {
	return telegramformat.Limit(value)
}

func trailingBackslashes(value []rune) int {
	count := 0
	for index := len(value) - 1; index >= 0 && value[index] == '\\'; index-- {
		count++
	}
	return count
}
