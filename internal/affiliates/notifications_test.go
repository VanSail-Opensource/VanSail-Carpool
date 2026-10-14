package affiliates

import (
	"strings"
	"testing"

	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestAffiliateNotificationEscapesDynamicMarkdown(t *testing.T) {
	message := formatSuccess(jobpayload.AffiliateSuccess{Locale: LocaleEnglish, InviteeName: "a_[b]", SettledAt: "2026-08-20T00:00:00Z", CommissionMinor: 105, TierName: "Pro!"})
	for _, expected := range []string{"a\\_\\[b\\]", "Pro\\!", "1\\.05"} {
		if !strings.Contains(message, expected) {
			t.Errorf("message %q does not contain %q", message, expected)
		}
	}
}
