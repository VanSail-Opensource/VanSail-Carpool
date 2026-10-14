package notifications

import (
	"strings"
	"testing"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestFormatAutomaticResetNoticesAreLocalizedAndDetailed(t *testing.T) {
	t.Parallel()
	when := "2026-08-21T00:00:00Z"
	tests := []struct {
		kind, englishTitle, chineseTitle string
		facts                            map[string]string
	}{
		{jobpayload.UserEventAutomaticReset, "Automatic traffic reset completed", "自动流量重置已完成",
			map[string]string{FactCombo: "Pro", FactUsed: "991", FactTrafficLimit: "1000", FactReset: "DAY",
				FactCharge: "100", FactBalance: "200", FactTime: when}},
		{jobpayload.UserEventAutomaticResetInsufficient, "Automatic traffic reset disabled", "自动流量重置已关闭",
			map[string]string{FactCombo: "Pro", FactUsed: "991", FactTrafficLimit: "1000", FactReset: "DAY",
				FactCharge: "300", FactBalance: "200", FactAutomationState: "disabled", FactTime: when}},
		{jobpayload.UserEventAutomaticResetFailed, "Automatic traffic reset refunded", "自动流量重置已退款",
			map[string]string{FactCombo: "Pro", FactAmount: "100", FactBalance: "200", FactReason: "rejected", FactTime: when}},
	}
	for _, test := range tests {
		for _, locale := range []struct{ code, title string }{{"en", test.englishTitle}, {"zh-CN", test.chineseTitle}} {
			message, err := Format(notificationFixture(test.kind, locale.code, test.facts), time.UTC)
			if err != nil {
				t.Fatalf("Format(%s,%s): %v", test.kind, locale.code, err)
			}
			usageMissing := test.kind != jobpayload.UserEventAutomaticResetFailed && !strings.Contains(message, `99\.1%`)
			if !strings.Contains(message, locale.title) || usageMissing {
				t.Fatalf("Format(%s,%s) = %q", test.kind, locale.code, message)
			}
		}
	}
}
