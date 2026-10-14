package app

import (
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestAbuseMessageUsesRecipientLocaleAndEscapesValues(t *testing.T) {
	for _, test := range []struct{ locale, title, action, warning string }{
		{"zh-CN", "⚠️ *滥用检测*", "处罚: 警告", "请立即停止您的滥用行为，否则可能被封禁且不予退款。"},
		{"en", "⚠️ *Abuse detected*", "Action: Warning", "Stop abusive activity now"},
	} {
		delivery := database.AbuseDelivery{
			AbuseJob: database.AbuseJob{Reason: "rule_[1]", Action: abuse.ActionWarning, QPS: 50, Limit: 30},
			Locale:   test.locale, Username: "a_*", OccurredAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		}
		message := abuseMessage(delivery)
		for _, want := range []string{test.title, test.action, test.warning, `a\_\*`, `rule\_\[1\]`} {
			if !strings.Contains(message, want) {
				t.Errorf("%s missing %q in %q", test.locale, want, message)
			}
		}
	}
}
