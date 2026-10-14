package app

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/telegramformat"
)

func TestDrawAnnouncementKeepsEntryAndProgressWithinTelegramLimit(t *testing.T) {
	t.Parallel()
	prizes := make([]activity.PrizeInput, 100)
	for index := range prizes {
		prizes[index] = activity.PrizeInput{Name: strings.Repeat("Long prize ", 8), Stock: 1}
	}
	draw := activity.LuckyDraw{LuckyDrawInput: activity.LuckyDrawInput{
		Name: "Raffle", Kind: "raffle", FeeMinor: 100, Threshold: 100,
		Keyword: "join", Command: "draw_now", Prizes: prizes,
	}}
	message := drawAnnouncement(draw, 73)
	if utf8.RuneCountInString(message) > telegramformat.MessageLimit ||
		!strings.Contains(message, "73 / 100") || !strings.Contains(message, "/draw\\_now") {
		t.Fatalf("announcement lost its core details or exceeded Telegram limit: %q", message)
	}
}
