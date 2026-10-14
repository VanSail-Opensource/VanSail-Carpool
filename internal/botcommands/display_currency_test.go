package botcommands

import (
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestGroupCommandFormattersUseSelectedCurrency(t *testing.T) {
	t.Parallel()
	copy := Text(English)
	cny := model.Money{Currency: "CNY", Minor: "125", Display: "1.25 CNY"}
	usd := model.Money{Currency: "USD", Minor: "50", Display: "0.50 USD"}

	if message := FormatBalance(copy, cny); !strings.Contains(message, "1\\.25 CNY") || strings.Contains(message, "TXB") {
		t.Fatalf("FormatBalance() = %q", message)
	}
	checkIn := FormatCheckInWithMoney(copy, activity.DailyCheckIn{RewardMinor: 50, BalanceAfterMinor: 125}, nil, usd, cny)
	if !strings.Contains(checkIn, "\\+0\\.50 USD") || !strings.Contains(checkIn, "1\\.25 CNY") || strings.Contains(checkIn, "TXB") {
		t.Fatalf("FormatCheckInWithMoney() = %q", checkIn)
	}
	rollover := RolloverSummary{State: RolloverPredicted, Amount: &usd}
	combo := FormatCombo(copy, markdownPurchase(), nil, rollover)
	if !strings.Contains(combo, "0\\.50 USD") || strings.Contains(combo, "TXB") {
		t.Fatalf("FormatCombo() = %q", combo)
	}
	deduct := FormatDeductSucceeded(copy, cny)
	if !strings.Contains(deduct, "1\\.25 CNY") || strings.Contains(deduct, "TXB") {
		t.Fatalf("FormatDeductSucceeded() = %q", deduct)
	}
}
