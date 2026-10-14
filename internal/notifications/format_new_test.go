package notifications

import (
	"strings"
	"testing"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestNewNotificationFormatsBothLocales(t *testing.T) {
	when := "2026-09-23T00:00:00Z"
	tests := []struct {
		kind   string
		facts  map[string]string
		titles [2]string
	}{
		{jobpayload.UserEventPurchaseQueued, map[string]string{FactCombo: "Pro_[1]", FactCharge: "1250", FactBalance: "5000", FactValidFrom: when, FactValidUntil: when}, [2]string{"Combo purchase scheduled", "套餐购买已排队"}},
		{jobpayload.UserEventRenewalScheduled, map[string]string{FactCombo: "Pro", FactTermCount: "2", FactCharge: "2500", FactBalance: "5000", FactValidFrom: when, FactValidUntil: when}, [2]string{"Renewal scheduled", "续费已安排"}},
		{jobpayload.UserEventAddonActivated, map[string]string{FactCombo: "Pro", FactAddOns: "Squad_[1]", FactCharge: "200", FactBalance: "5000", FactValidUntil: when, FactTime: when}, [2]string{"Squads added", "节点组已添加"}},
		{jobpayload.UserEventQueuedCancellation, map[string]string{FactCombo: "Pro", FactAmount: "1250", FactBalance: "6250", FactTime: when}, [2]string{"Queued combo cancelled", "排队套餐已取消"}},
		{jobpayload.UserEventAutoRenewalFailed, map[string]string{FactCombo: "Pro", FactExpired: when, FactReason: "INSUFFICIENT_BALANCE"}, [2]string{"Automatic renewal did not complete", "自动续费未完成"}},
		{jobpayload.UserEventManualResetCompleted, map[string]string{FactCombo: "Pro", FactCharge: "100", FactBalance: "5000", FactTime: when}, [2]string{"Traffic reset completed", "流量重置已完成"}},
		{jobpayload.UserEventManualResetRefunded, map[string]string{FactCombo: "Pro", FactAmount: "100", FactBalance: "5100", FactReason: "RESET_REJECTED", FactTime: when}, [2]string{"Traffic reset refunded", "流量重置已退款"}},
		{jobpayload.UserEventMemberRefundCompleted, map[string]string{FactCombo: "Pro", FactAmount: "1250", FactBalance: "6250", FactTime: when}, [2]string{"Combo refunded", "套餐已退款"}},
		{jobpayload.UserEventMemberRefundFailed, map[string]string{FactCombo: "Pro", FactReason: "REFUND_TRAFFIC_USED", FactTime: when}, [2]string{"Combo refund did not complete", "套餐退款未完成"}},
		{jobpayload.UserEventProvisionConflict, map[string]string{FactCancelledCombos: "Pro_[1]", FactAmount: "1250", FactBalance: "5000", FactTime: when}, [2]string{"Combo access refunded", "套餐费用已退还"}},
	}
	for _, test := range tests {
		for index, locale := range []string{"en", "zh-CN"} {
			message, err := Format(notificationFixture(test.kind, locale, test.facts), time.UTC)
			if err != nil {
				t.Fatalf("Format(%s,%s): %v", test.kind, locale, err)
			}
			if !strings.Contains(message, test.titles[index]) {
				t.Errorf("%s %s missing title: %q", locale, test.kind, message)
			}
			if strings.Contains(message, "INSUFFICIENT_BALANCE") || strings.Contains(message, "REFUND_TRAFFIC_USED") {
				t.Errorf("raw reason leaked: %q", message)
			}
			if strings.Contains(message, "Pro") && strings.Contains(test.facts[FactCombo], "_") && !strings.Contains(message, `Pro\_\[1\]`) {
				t.Errorf("combo Markdown was not escaped: %q", message)
			}
		}
	}
}
