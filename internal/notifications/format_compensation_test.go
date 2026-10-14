package notifications

import (
	"strings"
	"testing"
	"time"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestFormatNodeCompensationDetails(t *testing.T) {
	t.Parallel()
	facts := map[string]string{
		FactNode:               "Edge_[A]",
		FactAffectedSquads:     "Core + Plus, Night_[Ops]",
		FactDowntimeSeconds:    "7500",
		FactOutageStarted:      "2026-08-23T00:00:00Z",
		FactRecovered:          "2026-08-23T02:05:00Z",
		FactAddedSeconds:       "3900",
		FactCompensationCapped: "true",
		FactCombo:              "Pro_[1]",
		FactPreviousExpiry:     "2026-09-01T00:00:00Z",
		FactNewExpiry:          "2026-09-01T01:05:00Z",
		FactReason:             "Reviewed_[outage]!",
		FactTime:               "2026-08-23T03:00:00Z",
	}
	tests := []struct {
		locale   string
		contains []string
	}{
		{locale: "en", contains: []string{
			"🎁 *Node outage compensation received*", "*Node:* Edge\\_\\[A\\]",
			"*Affected squads:* Core \\+ Plus, Night\\_\\[Ops\\]", "*Outage duration:* 2 hours 5 minutes",
			"*Compensation:* 1 hour 5 minutes", "*Calculation capped:* Yes",
			"*Reason:* Reviewed\\_\\[outage\\]\\!", "*Applied at:* 2026\\-08\\-23 11:00 CST",
		}},
		{locale: "zh-CN", contains: []string{
			"🎁 *节点故障补偿已到账*", "*受影响节点组:* Core \\+ Plus, Night\\_\\[Ops\\]",
			"*故障时长:* 2 小时 5 分钟", "*补偿时长:* 1 小时 5 分钟", "*计算结果已封顶:* 是",
		}},
	}
	location := time.FixedZone("CST", 8*60*60)
	for _, test := range tests {
		t.Run(test.locale, func(t *testing.T) {
			message, err := Format(notificationFixture(jobpayload.UserEventNodeCompensation, test.locale, facts), location)
			if err != nil {
				t.Fatalf("Format(): %v", err)
			}
			for _, expected := range test.contains {
				if !strings.Contains(message, expected) {
					t.Errorf("message %q does not contain %q", message, expected)
				}
			}
		})
	}
}
