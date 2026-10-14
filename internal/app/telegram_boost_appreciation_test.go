package app

import (
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestBoostAppreciationUsesEscapedIdentityAndLocalizedFallback(t *testing.T) {
	for _, test := range []struct {
		item model.TelegramBoostAppreciation
		want string
	}{
		{model.TelegramBoostAppreciation{Username: "test_user", Name: "ignored"}, `@test\_user`},
		{model.TelegramBoostAppreciation{Name: "Mira [TX]", Locale: "zh-CN"}, `感谢 Mira \[TX\]`},
		{model.TelegramBoostAppreciation{}, "Thank you for boosting"},
	} {
		if got := boostAppreciationText(test.item); !strings.Contains(got, test.want) {
			t.Fatalf("appreciation = %q", got)
		}
	}
}
