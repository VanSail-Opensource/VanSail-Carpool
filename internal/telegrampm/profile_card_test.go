package telegrampm

import (
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestProfileHidesAccountFactsBeforeOnboardingCompletes(t *testing.T) {
	item := model.PMConversation{ID: "conversation", UserID: "user-1", TelegramID: 42, ChatID: -100123, TopicID: 55, Locale: "en"}
	active := &model.Purchase{ComboName: "Core", Price: model.TXBMoney(10000)}
	card := Profile(item, ProfileFacts{OnboardingState: "agreement", Balance: model.TXBMoney(5000), ActiveCombo: active}, "https://panel.example", "UTC")
	if !strings.Contains(card.Text, "Not yet onboarded") || strings.Contains(card.Text, "Balance") || strings.Contains(card.Text, "Core") {
		t.Fatalf("incomplete profile card leaked account facts: %s", card.Text)
	}
	if len(card.ReplyMarkup.InlineKeyboard) != 3 || card.ReplyMarkup.InlineKeyboard[2][0].URL == "" {
		t.Fatalf("profile controls missing: %+v", card.ReplyMarkup)
	}
}

func TestProfileShowsApplicableFactsAndOmittedCoupon(t *testing.T) {
	item := model.PMConversation{ID: "conversation", UserID: "user-1", TelegramID: 42, ChatID: -100123, TopicID: 55, Locale: "en"}
	used, allocated := int64(1<<30), int64(8<<30)
	purchase := &model.Purchase{ComboName: "Core", ValidFrom: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), ValidUntil: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), AutoRenewEnabled: true, CouponDiscount: model.TXBMoney(2500)}
	card := Profile(item, ProfileFacts{OnboardingState: "complete", Balance: model.TXBMoney(10000), ActiveCombo: purchase,
		SquadNames: []string{"Optional squad"}, UsedBytes: &used, AllocatedBytes: &allocated}, "https://panel.example", "UTC")
	for _, want := range []string{"Balance", "Core", "Optional squad", "2026\\-10\\-01", "Auto renew", "1\\.00 GiB / 8\\.00 GiB"} {
		if !strings.Contains(card.Text, want) {
			t.Fatalf("profile card missing %q: %s", want, card.Text)
		}
	}
	if strings.Contains(card.Text, "Coupon discount") {
		t.Fatalf("unused coupon discount was shown: %s", card.Text)
	}
	grantID := "coupon-grant"
	purchase.CouponGrantID = &grantID
	withCoupon := Profile(item, ProfileFacts{OnboardingState: "complete", Balance: model.TXBMoney(10000), ActiveCombo: purchase}, "https://panel.example", "UTC")
	if !strings.Contains(withCoupon.Text, "Coupon discount") {
		t.Fatalf("used coupon discount was omitted: %s", withCoupon.Text)
	}
	if card.ReplyMarkup.InlineKeyboard[2][0].URL != "https://panel.example/admin/users/user-1" {
		t.Fatalf("Browser profile link = %q", card.ReplyMarkup.InlineKeyboard[2][0].URL)
	}
}

func TestBrowserProfileURLContainsNoAuthorizationMaterial(t *testing.T) {
	for _, test := range []struct{ base, user, want string }{
		{"https://panel.example", "user-id", "https://panel.example/admin/users/user-id"},
		{"https://panel.example/base", "user-id", "https://panel.example/admin/users/user-id"},
		{"http://localhost:8080", "user-id", "http://localhost:8080/admin/users/user-id"},
		{"https://user:secret@panel.example", "user-id", ""},
		{"https://panel.example?token=secret", "user-id", ""},
		{"https://panel.example#token=secret", "user-id", ""},
		{"https://panel.example", "../user-id", ""},
		{"https://panel.example", "user/id", ""},
		{"https://panel.example", "", ""},
		{"", "user-id", ""},
	} {
		t.Run(test.base+test.user, func(t *testing.T) {
			if got := browserProfileURL(test.base, test.user); got != test.want {
				t.Fatalf("profile URL=%q,want=%q", got, test.want)
			}
		})
	}
}

func TestProfileShowsPAYGUsageWithoutAnActiveCombo(t *testing.T) {
	for _, locale := range []string{"en", "zh-CN"} {
		item := model.PMConversation{ID: "payg-member", UserID: "42", TelegramID: 42, Locale: locale}
		card := Profile(item, ProfileFacts{OnboardingState: "complete", Balance: model.TXBMoney(500),
			PAYGTraffic: "1073741824", PAYGSpent: model.TXBMoney(125)}, "https://panel.example", "UTC")
		if !strings.Contains(card.Text, "PAYG") || !strings.Contains(card.Text, "1\\.00 GiB") || !strings.Contains(card.Text, "1\\.25 TXB") {
			t.Fatal("PAYG-only profile omitted accounting", locale, card.Text)
		}
		if strings.Contains(card.Text, "Proxy username") {
			t.Fatal("duplicate identity displayed", card.Text)
		}
	}
}
