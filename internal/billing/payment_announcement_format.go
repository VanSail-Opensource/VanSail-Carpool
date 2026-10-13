package billing

import (
	"errors"
	"strings"

	"github.com/txyyddss/Remna-User-Panel/internal/model"
	jobpayload "github.com/txyyddss/Remna-User-Panel/internal/outbox"
	"github.com/txyyddss/Remna-User-Panel/internal/telegramformat"
)

var paymentAnnouncementChannelsZH = map[string]string{
	"alipay":        "支付宝",
	"wxpay":         "微信支付",
	"wechat":        "微信支付",
	"qqpay":         "QQ 钱包",
	"bank":          "银联",
	"jdpay":         "京东支付",
	"usdt.trc20":    "USDT · TRC20",
	"usdt.erc20":    "USDT · ERC20",
	"usdt.polygon":  "USDT · Polygon",
	"usdt.bep20":    "USDT · BEP20",
	"usdt.aptos":    "USDT · Aptos",
	"usdt.solana":   "USDT · Solana",
	"usdt.xlayer":   "USDT · X Layer",
	"usdt.arbitrum": "USDT · Arbitrum",
	"usdt.plasma":   "USDT · Plasma",
	"usdt.ton":      "USDT · TON",
}

func formatPaymentSuccessAnnouncement(payload jobpayload.PaymentSuccessAnnouncement) string {
	providerName := strings.TrimSpace(payload.ProviderName)
	if providerName == "" {
		providerName = paymentAnnouncementProvider(payload.Provider)
	}
	lines := []string{
		"💰 收款到账 +" + payload.PayableAmount + payload.PayableCurrency,
		"============订单详情============",
		"提供商: " + providerName,
		"渠道: " + paymentAnnouncementChannel(payload.Provider, payload.Channel),
		"TXB金额: " + model.TXBMoney(payload.TXBMinor).Display,
		"用户名: " + payload.Username,
		"",
		"感谢您对TX的信任",
	}
	for index := range lines {
		lines[index] = markdownV2Escape(lines[index])
	}
	return telegramformat.Limit(strings.Join(lines, "\n"))
}

func markdownV2Escape(value string) string {
	return telegramformat.Escape(value)
}

func validatePaymentSuccessAnnouncement(payload jobpayload.PaymentSuccessAnnouncement) error {
	amount, err := ParseDecimal(payload.PayableAmount)
	if err != nil || !amount.Positive() {
		return errors.New("payment announcement payable amount is invalid")
	}
	expectedCurrency := strings.ToUpper(currencyCode(payload.Provider))
	if expectedCurrency == "" || payload.PayableCurrency != expectedCurrency {
		return errors.New("payment announcement provider currency is invalid")
	}
	if payload.Provider != "ezpay" && payload.Provider != "bepusdt" && payload.Provider != "stars" && payload.Provider != "builtin_usdt" {
		return errors.New("payment announcement provider is unsupported")
	}
	return nil
}

func paymentAnnouncementProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "ezpay":
		return "EZPay"
	case "bepusdt":
		return "BEPUSDT"
	case "builtin_usdt":
		return "USDT"
	case "stars":
		return "Telegram Stars"
	default:
		return strings.ToUpper(strings.TrimSpace(provider))
	}
}

func paymentAnnouncementChannel(provider, channel string) string {
	if provider == "stars" {
		return "Telegram Stars"
	}
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" {
		_, _, legacyRail, err := ParseMethodSelection(provider)
		if err == nil {
			channel = legacyRail
		}
	}
	if name := paymentAnnouncementChannelsZH[channel]; name != "" {
		return name
	}
	if channel != "" {
		return channel
	}
	return "默认"
}
