package notifications

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestFormatNotificationCards(t *testing.T) {
	t.Parallel()
	location := time.FixedZone("CST", 8*60*60)
	tests := []struct {
		name     string
		payload  jobpayload.UserNotification
		contains []string
	}{
		{name: "English renewal", payload: notificationFixture(jobpayload.UserEventAutoRenewal, "en", map[string]string{
			FactCombo: "Pro_[1]", FactRenewalDebit: "-1250", FactUsed: "1024", FactAllocated: "2048",
			FactEligible: "1024", FactRollover: "250", FactBalance: "5000", FactValidUntil: "2026-09-20T00:00:00Z",
		}), contains: []string{"♻️ *Auto\\-renewed*", "*Combo:* Pro\\_\\[1\\]", "*Renewal charge:* 12\\.50 TXB", "*Rollover:* 2\\.50 TXB"}},
		{name: "Chinese extension", payload: notificationFixture(jobpayload.UserEventAdminExtension, "zh-CN", map[string]string{
			FactAddedSeconds: "86400", FactPreviousExpiry: "2026-08-20T00:00:00Z", FactNewExpiry: "2026-08-21T00:00:00Z",
			FactReason: "补偿_[确认]!", FactTime: "2026-08-20T01:00:00Z",
		}), contains: []string{"🎁 *管理员已延长订阅*", "*延长:* 1 天", "*原因:* 补偿\\_\\[确认\\]\\!"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message, err := Format(test.payload, location)
			if err != nil {
				t.Fatalf("Format(): %v", err)
			}
			for _, expected := range test.contains {
				if !strings.Contains(message, expected) {
					t.Errorf("message %q does not contain %q", message, expected)
				}
			}
			if utf8.RuneCountInString(message) > 4096 {
				t.Fatalf("message has %d runes", utf8.RuneCountInString(message))
			}
		})
	}
}

func TestFormatKeepsMaximumAdminReason(t *testing.T) {
	payload := notificationFixture(jobpayload.UserEventAdminUpdate, "en", map[string]string{
		FactChange: "balance_adjustment", FactAmount: "100", FactBalance: "200",
		FactReason: strings.Repeat("x", 500), FactTime: "2026-08-20T00:00:00Z",
	})
	message, err := Format(payload, time.UTC)
	if err != nil {
		t.Fatalf("Format(): %v", err)
	}
	if !strings.Contains(message, strings.Repeat("x", 500)) {
		t.Fatal("Format() truncated the accepted administrator reason")
	}
}

func TestFormatUsesExactOrderedLocalizedCards(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	tests := []struct {
		name, want string
		payload    jobpayload.UserNotification
	}{
		{
			name: "English expiration",
			payload: notificationFixture(jobpayload.UserEventExpiration, "en", map[string]string{
				FactCombo: "Pro", FactExpired: "2026-08-20T00:00:00Z",
			}),
			want: "🛑 *Subscription expired*\n*Combo:* Pro\n*Expired:* 2026\\-08\\-20 08:00 CST\n\nOpen VanSail Carpool to restore access\\.",
		},
		{
			name: "Chinese reminder",
			payload: notificationFixture(jobpayload.UserEventExpiryReminder, "zh-CN", map[string]string{
				FactCombo: "专业版", FactExpires: "2026-08-20T00:00:00Z", FactAutoRenewal: "off", FactQueuedCombo: "none",
			}),
			want: "⏳ *订阅即将到期*\n*套餐:* 专业版\n*到期时间:* 2026\\-08\\-20 08:00 CST\n*自动续费:* 关闭\n*排队套餐:* 无\n\n请在到期前前往 VanSail Carpool 选择新套餐。",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Format(test.payload, location)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("Format() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestFormatShowsUnavailableRollover(t *testing.T) {
	payload := notificationFixture(jobpayload.UserEventAutoRenewal, "en", map[string]string{
		FactCombo: "Pro", FactRenewalDebit: "-1250", FactUsed: "0", FactAllocated: "0", FactEligible: "0",
		FactRollover: "0", FactRolloverStatus: "unavailable", FactBalance: "5000", FactValidUntil: "2026-09-20T00:00:00Z",
	})
	message, err := Format(payload, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "*Rollover:* Unavailable") {
		t.Fatalf("Format() = %q", message)
	}
}

func TestFormatHonorsTelegramBoundary(t *testing.T) {
	payload := notificationFixture(jobpayload.UserEventExpiration, "en", map[string]string{
		FactCombo: strings.Repeat("x", 5000), FactExpired: "2026-08-20T00:00:00Z",
	})
	message, err := Format(payload, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if utf8.RuneCountInString(message) != 4096 || !strings.HasSuffix(message, `\.\.\.`) {
		t.Fatalf("bounded message has %d runes and suffix %q", utf8.RuneCountInString(message), message[len(message)-6:])
	}
}

func TestFormatUsesEveryExactLocalizedTitle(t *testing.T) {
	titles := map[string][2]string{
		jobpayload.UserEventExpiration:         {"🛑 *Subscription expired*", "🛑 *订阅已到期*"},
		jobpayload.UserEventExpiryReminder:     {"⏳ *Subscription expiring soon*", "⏳ *订阅即将到期*"},
		jobpayload.UserEventQueuedActivation:   {"🚀 *Queued combo activated*", "🚀 *排队套餐已启用*"},
		jobpayload.UserEventPurchaseActivation: {"✅ *Combo activated*", "✅ *套餐已启用*"},
		jobpayload.UserEventAutoRenewal:        {"♻️ *Auto\\-renewed*", "♻️ *自动续费成功*"},
		jobpayload.UserEventPaymentCredited:    {"💰 *TXB added to your balance*", "💰 *TXB 已到账*"},
		jobpayload.UserEventPaymentRefunded:    {"↩️ *Payment refunded*", "↩️ *支付已退款*"},
		jobpayload.UserEventTrafficThreshold:   {"⚠️ *Traffic above 90%*", "⚠️ *流量已超过 90%*"},
		jobpayload.UserEventGroupReward:        {"🎁 *Group reward received*", "🎁 *群聊奖励到账*"},
		jobpayload.UserEventAdminExtension:     {"🎁 *Extended by admin*", "🎁 *管理员已延长订阅*"},
		jobpayload.UserEventAdminUpdate:        {"🛠 *Balance adjusted by admin*", "🛠 *管理员已调整余额*"},
		jobpayload.UserEventNodeCompensation:   {"🎁 *Node outage compensation received*", "🎁 *节点故障补偿已到账*"},
	}
	for kind, expected := range titles {
		for index, locale := range []string{"en", "zh-CN"} {
			message, err := Format(notificationFixture(kind, locale, factsForKind(kind)), time.UTC)
			if err != nil {
				t.Fatalf("Format(%s,%s): %v", kind, locale, err)
			}
			if first := strings.SplitN(message, "\n", 2)[0]; first != expected[index] {
				t.Fatalf("Format(%s,%s) title = %q, want %q", kind, locale, first, expected[index])
			}
		}
	}
}

func factsForKind(kind string) map[string]string {
	when := "2026-08-20T00:00:00Z"
	switch kind {
	case jobpayload.UserEventExpiration:
		return map[string]string{FactCombo: "Pro", FactExpired: when}
	case jobpayload.UserEventExpiryReminder:
		return map[string]string{FactCombo: "Pro", FactExpires: when, FactAutoRenewal: "off", FactQueuedCombo: "none"}
	case jobpayload.UserEventQueuedActivation:
		return map[string]string{FactCombo: "Pro", FactTrafficLimit: "1024", FactReset: "DAY", FactValidUntil: when}
	case jobpayload.UserEventPurchaseActivation:
		return map[string]string{FactCombo: "Pro", FactCharge: "100", FactTrafficLimit: "1024", FactReset: "DAY", FactValidUntil: when}
	case jobpayload.UserEventAutoRenewal:
		return map[string]string{FactCombo: "Pro", FactRenewalDebit: "-100", FactUsed: "0", FactAllocated: "1024",
			FactEligible: "1024", FactRollover: "100", FactBalance: "200", FactValidUntil: when}
	case jobpayload.UserEventTrafficThreshold:
		return map[string]string{FactCombo: "Pro", FactUsed: "901", FactTrafficLimit: "1000", FactRemaining: "99", FactReset: "DAY"}
	case jobpayload.UserEventPaymentCredited:
		return map[string]string{FactProvider: "ezpay", FactPaymentAmount: "10.00 CNY", FactAmount: "2500", FactBalance: "2500", FactTime: when}
	case jobpayload.UserEventPaymentRefunded:
		return map[string]string{FactProvider: "stars", FactAmount: "2500", FactBalance: "0", FactTime: when}
	case jobpayload.UserEventGroupReward:
		return map[string]string{FactMessages: "10", FactReward: "100", FactBalance: "200", FactTime: when}
	case jobpayload.UserEventAdminExtension:
		return map[string]string{FactAddedSeconds: "86400", FactPreviousExpiry: when,
			FactNewExpiry: "2026-08-21T00:00:00Z", FactReason: "reason", FactTime: when}
	case jobpayload.UserEventNodeCompensation:
		return map[string]string{FactNode: "Node", FactAffectedSquads: "Core", FactDowntimeSeconds: "120",
			FactOutageStarted: when, FactRecovered: "2026-08-20T00:02:00Z", FactAddedSeconds: "60", FactCombo: "Pro",
			FactPreviousExpiry: when, FactNewExpiry: "2026-08-20T00:01:00Z", FactReason: "reason", FactTime: when}
	default:
		return map[string]string{FactChange: "balance_adjustment", FactAmount: "100", FactBalance: "200",
			FactReason: "reason", FactTime: when}
	}
}

func notificationFixture(kind, locale string, facts map[string]string) jobpayload.UserNotification {
	return jobpayload.UserNotification{EventKey: "event-1", UserID: "user-1", ChatID: 42, Locale: locale,
		Kind: kind, OccurredAt: "2026-08-20T00:00:00Z", Facts: facts}
}
