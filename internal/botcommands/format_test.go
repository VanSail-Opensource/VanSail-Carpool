package botcommands

import (
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestFormatStartAndBalanceExactOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		language Language
		start    string
		balance  string
	}{
		{name: "English", language: English, start: `Press Open VanSail Carpool to use the app\. For help, send a message directly here after opening the panel and completing entry verification\.`, balance: "Balance: 12\\.50 TXB"},
		{name: "Chinese", language: Chinese, start: "请按“打开 VanSail Carpool”使用应用。如需帮助，请先打开面板并完成首次访问验证，再直接在这里发送消息。", balance: "余额: 12\\.50 TXB"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			copy := Text(test.language)
			if got := FormatStart(copy); got != test.start {
				t.Errorf("FormatStart() = %q, want %q", got, test.start)
			}
			if got := FormatBalance(copy, model.TXBMoney(1_250)); got != test.balance {
				t.Errorf("FormatBalance() = %q, want %q", got, test.balance)
			}
		})
	}
}

func TestFormatCheckInComparisonStates(t *testing.T) {
	t.Parallel()
	average := int64(50)
	for _, language := range []Language{English, Chinese} {
		copy := Text(language)
		tests := []struct {
			name    string
			result  activity.DailyCheckIn
			average *int64
			tone    string
		}{
			{name: "above", result: activity.DailyCheckIn{RewardMinor: 51, BalanceAfterMinor: 151}, average: &average, tone: copy.SignInAbove},
			{name: "below", result: activity.DailyCheckIn{RewardMinor: 49, BalanceAfterMinor: 149}, average: &average, tone: copy.SignInBelow},
			{name: "equal", result: activity.DailyCheckIn{RewardMinor: 50, BalanceAfterMinor: 150}, average: &average, tone: copy.SignInEqual},
			{name: "unavailable", result: activity.DailyCheckIn{RewardMinor: 50, BalanceAfterMinor: 150}, tone: copy.SignInNeutral},
			{name: "already claimed", result: activity.DailyCheckIn{RewardMinor: 50, BalanceAfterMinor: 150, AlreadyClaimed: true}, average: &average, tone: copy.SignInAlready},
		}
		for _, test := range tests {
			t.Run(string(language)+"/"+test.name, func(t *testing.T) {
				got := FormatCheckIn(copy, test.result, test.average)
				want := escapeMarkdownV2(test.tone) + "\n🎁 *" + escapeMarkdownV2(copy.SignInReward) + ":* " +
					escapeMarkdownV2("+"+model.TXBMoney(test.result.RewardMinor).Display) + "\n💰 *" +
					escapeMarkdownV2(copy.BalanceLabel) + ":* " + escapeMarkdownV2(model.TXBMoney(test.result.BalanceAfterMinor).Display)
				if got != want {
					t.Errorf("FormatCheckIn() = %q, want %q", got, want)
				}
			})
		}
	}
}

func TestFormatComboRolloverStates(t *testing.T) {
	t.Parallel()
	amount := model.TXBMoney(1_234)
	for _, language := range []Language{English, Chinese} {
		copy := Text(language)
		tests := []struct {
			name    string
			summary RolloverSummary
			want    string
		}{
			{name: "predicted", summary: RolloverSummary{State: RolloverPredicted, Amount: &amount}, want: copy.RolloverWill + " " + amount.Display},
			{name: "ineligible", summary: RolloverSummary{State: RolloverIneligible}, want: copy.RolloverWillNot},
			{name: "disabled", summary: RolloverSummary{State: RolloverDisabled}, want: copy.RolloverCannot},
			{name: "unavailable", summary: RolloverSummary{State: RolloverUnavailable}, want: copy.RolloverUnavailable},
			{name: "missing predicted amount", summary: RolloverSummary{State: RolloverPredicted}, want: copy.RolloverUnavailable},
		}
		for _, test := range tests {
			t.Run(string(language)+"/"+test.name, func(t *testing.T) {
				got := FormatCombo(copy, markdownPurchase(), []string{"US"}, test.summary)
				if !strings.Contains(got, "♻️ *"+escapeMarkdownV2(copy.RolloverLabel)+":* "+escapeMarkdownV2(test.want)) {
					t.Errorf("FormatCombo() = %q, want rollover %q", got, test.want)
				}
				if !strings.Contains(got, escapeMarkdownV2(copy.ResetMonthly)) {
					t.Errorf("FormatCombo() = %q, want localized cadence", got)
				}
			})
		}
	}
}

func TestFormattersEscapeMarkdownV2DynamicValues(t *testing.T) {
	t.Parallel()
	copy := Text(English)
	replies := []string{
		FormatCombo(copy, markdownPurchase(), []string{markdownValue}, RolloverSummary{State: RolloverIneligible}),
		FormatSubscription(copy, markdownDashboard(), time.Now().UTC()),
		FormatWelcome(copy, 12345, markdownValue),
	}
	for _, reply := range replies {
		for _, escaped := range []string{`\_`, `\*`, `\[`, `\]`, `\(`, `\)`, `\~`, "\\`", `\>`, `\#`, `\+`, `\-`, `\=`, `\|`, `\{`, `\}`, `\.`, `\!`, `\\`} {
			if !strings.Contains(reply, escaped) {
				t.Fatalf("reply does not escape %q: %q", escaped, reply)
			}
		}
	}
}

func TestFormatWelcomeUsesLocalizedSafeMention(t *testing.T) {
	t.Parallel()
	tests := []struct {
		language Language
		want     string
	}{
		{language: English, want: `👋 Welcome [Ada \_\*](tg://user?id=12345) to VanSail Carpool\!`},
		{language: Chinese, want: `👋 欢迎 [Ada \_\*](tg://user?id=12345) 加入 VanSail Carpool！`},
	}
	for _, test := range tests {
		if got := FormatWelcome(Text(test.language), 12345, "Ada _*"); got != test.want {
			t.Errorf("FormatWelcome(%s) = %q, want %q", test.language, got, test.want)
		}
	}
}

func TestLimitMarkdownV2KeepsEscapesComplete(t *testing.T) {
	t.Parallel()
	result := Limit(strings.Repeat("a", MessageLimit-7) + "\\" + strings.Repeat("x", 10))
	withoutEllipsis := strings.TrimSuffix(result, markdownV2Ellipsis)
	if len([]rune(result)) > MessageLimit || !strings.HasSuffix(result, markdownV2Ellipsis) {
		t.Fatalf("limited reply = %q", result)
	}
	if trailingBackslashes([]rune(withoutEllipsis))%2 != 0 {
		t.Fatalf("limited reply ends with an incomplete escape: %q", result)
	}
}

const markdownValue = "edge_*[]()~`>#+-=|{}.!\\"

func markdownDashboard() model.Dashboard {
	return model.Dashboard{
		ActivePurchase: markdownPurchase(),
		Statistics: &model.Statistics{
			UsedTrafficBytes:  "1024",
			TrafficLimitBytes: "2048",
			TopNodes: []model.TopNode{{
				Name: markdownValue, CountryCode: "us", TotalBytes: "1024",
			}},
		},
	}
}

func markdownPurchase() *model.Purchase {
	return &model.Purchase{ComboName: markdownValue, TrafficLimitBytes: 1024, ResetStrategy: "MONTH_ROLLING"}
}
