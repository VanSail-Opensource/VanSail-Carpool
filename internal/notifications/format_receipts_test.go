package notifications

import (
	"strings"
	"testing"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestFormatNewMemberReceipts(t *testing.T) {
	t.Parallel()
	when := "2026-08-20T00:00:00Z"
	tests := []struct {
		name, kind, locale string
		facts              map[string]string
		want               []string
	}{
		{"purchase en", jobpayload.UserEventPurchaseActivation, "en",
			map[string]string{FactCombo: "Pro_[1]", FactCharge: "1250", FactTrafficLimit: "1024", FactReset: "DAY", FactValidUntil: when},
			[]string{"✅ *Combo activated*", "*Combo:* Pro\\_\\[1\\]", "*Charged:* 12\\.50 TXB", "*Reset:* Daily"}},
		{"purchase zh", jobpayload.UserEventPurchaseActivation, "zh-CN",
			map[string]string{FactCombo: "专业版", FactCharge: "1250", FactTrafficLimit: "1024", FactReset: "DAY", FactValidUntil: when},
			[]string{"✅ *套餐已启用*", "*已扣款:* 12\\.50 TXB", "*重置:* 每日"}},
		{"payment en", jobpayload.UserEventPaymentCredited, "en",
			map[string]string{FactProvider: "ezpay", FactPaymentAmount: "10.00 CNY", FactAmount: "2500", FactBalance: "2500", FactTime: when},
			[]string{"💰 *TXB added to your balance*", "*Provider:* EZPay", "*Paid:* 10\\.00 CNY", "*TXB credited:* 25\\.00 TXB"}},
		{"payment zh", jobpayload.UserEventPaymentCredited, "zh-CN",
			map[string]string{FactProvider: "stars", FactPaymentAmount: "100 XTR", FactAmount: "2500", FactBalance: "2500", FactTime: when},
			[]string{"💰 *TXB 已到账*", "*支付服务商:* Telegram Stars", "*支付金额:* 100 XTR", "*TXB 入账:* 25\\.00 TXB"}},
		{"refund en", jobpayload.UserEventPaymentRefunded, "en",
			map[string]string{FactProvider: "stars", FactAmount: "2500", FactBalance: "0", FactCancelledCombos: "Pro_[1]", FactTime: when},
			[]string{"↩️ *Payment refunded*", "*TXB deducted:* 25\\.00 TXB", "*Cancelled combos:* Pro\\_\\[1\\]"}},
		{"refund zh", jobpayload.UserEventPaymentRefunded, "zh-CN",
			map[string]string{FactProvider: "stars", FactAmount: "2500", FactBalance: "0", FactTime: when},
			[]string{"↩️ *支付已退款*", "*TXB 扣回:* 25\\.00 TXB", "*余额:* 0\\.00 TXB"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message, err := Format(notificationFixture(test.kind, test.locale, test.facts), time.UTC)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range test.want {
				if !strings.Contains(message, want) {
					t.Errorf("message %q does not contain %q", message, want)
				}
			}
		})
	}
}

func TestResetRefundUsesLocalizedReason(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ locale, reason, want string }{
		{"en", "RESET_REJECTED", "The provider rejected the reset"},
		{"zh-CN", "RESET_PRECHECK_FAILED", "服务商未能核验订阅"},
		{"en", "SOME_FUTURE_CODE", "The reset could not be completed"},
	} {
		t.Run(test.locale+test.reason, func(t *testing.T) {
			message, err := Format(notificationFixture(jobpayload.UserEventAutomaticResetFailed, test.locale, map[string]string{
				FactCombo: "Pro", FactAmount: "100", FactBalance: "200", FactReason: test.reason,
				FactTime: "2026-08-20T00:00:00Z",
			}), time.UTC)
			if err != nil || !strings.Contains(message, test.want) || strings.Contains(message, test.reason) {
				t.Fatalf("Format() = %q, %v", message, err)
			}
		})
	}
}
