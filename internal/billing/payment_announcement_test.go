package billing

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestPaymentAnnouncementFormatting(t *testing.T) {
	t.Parallel()
	payload := paymentAnnouncementFixture("ezpay", "alipay", "12.34", "CNY")
	payload.ProviderName = "收款_一号"
	want := `💰 收款到账 \+12\.34CNY
\=\=\=\=\=\=\=\=\=\=\=\=订单详情\=\=\=\=\=\=\=\=\=\=\=\=
提供商: 收款\_一号
渠道: 支付宝
TXB金额: 25\.00 TXB
用户名: @ada

感谢您对TX的信任`
	if got := formatPaymentSuccessAnnouncement(payload); got != want {
		t.Errorf("formatPaymentSuccessAnnouncement() = %q, want %q", got, want)
	}
}

func TestPaymentAnnouncementLocalizesEveryChannel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		provider string
		channel  string
		want     string
	}{
		{provider: "ezpay", channel: "alipay", want: "支付宝"},
		{provider: "ezpay", channel: "wxpay", want: "微信支付"},
		{provider: "ezpay", channel: "wechat", want: "微信支付"},
		{provider: "ezpay", channel: "qqpay", want: "QQ 钱包"},
		{provider: "ezpay", channel: "bank", want: "银联"},
		{provider: "ezpay", channel: "jdpay", want: "京东支付"},
		{provider: "bepusdt", channel: "usdt.trc20", want: "USDT · TRC20"},
		{provider: "bepusdt", channel: "usdt.erc20", want: "USDT · ERC20"},
		{provider: "bepusdt", channel: "usdt.polygon", want: "USDT · Polygon"},
		{provider: "bepusdt", channel: "usdt.bep20", want: "USDT · BEP20"},
		{provider: "bepusdt", channel: "usdt.aptos", want: "USDT · Aptos"},
		{provider: "bepusdt", channel: "usdt.solana", want: "USDT · Solana"},
		{provider: "bepusdt", channel: "usdt.xlayer", want: "USDT · X Layer"},
		{provider: "bepusdt", channel: "usdt.arbitrum", want: "USDT · Arbitrum"},
		{provider: "bepusdt", channel: "usdt.plasma", want: "USDT · Plasma"},
		{provider: "bepusdt", channel: "usdt.ton", want: "USDT · TON"},
		{provider: "stars", want: "Telegram Stars"},
		{provider: "ezpay", want: "支付宝"},
		{provider: "bepusdt", want: "USDT · TRC20"},
	}
	for _, test := range tests {
		t.Run(test.provider+"/"+test.channel, func(t *testing.T) {
			if got := paymentAnnouncementChannel(test.provider, test.channel); got != test.want {
				t.Errorf("paymentAnnouncementChannel() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPaymentAnnouncementFallbackAndMarkdownEscaping(t *testing.T) {
	t.Parallel()
	payload := paymentAnnouncementFixture("ezpay", "fee_test", "12.34", "CNY")
	payload.ProviderName = `Admin_*[]()~` + "`" + `>#+-=|{}.!\`
	payload.Username = `user_*[]()~` + "`" + `>#+-=|{}.!\`
	message := formatPaymentSuccessAnnouncement(payload)
	for _, escaped := range []string{`\_`, `\*`, `\[`, `\]`, `\(`, `\)`, `\~`, "\\`", `\>`, `\#`, `\+`, `\-`, `\=`, `\|`, `\{`, `\}`, `\.`, `\!`, `\\`} {
		if !strings.Contains(message, escaped) {
			t.Errorf("announcement does not escape %q: %q", escaped, message)
		}
	}
	legacy := paymentAnnouncementFixture("stars", "", "7", "XTR")
	if message := formatPaymentSuccessAnnouncement(legacy); !strings.Contains(message, "提供商: Telegram Stars") || !strings.Contains(message, "渠道: Telegram Stars") {
		t.Errorf("legacy fallback announcement = %q", message)
	}
	legacy = paymentAnnouncementFixture("ezpay", "", "12.34", "CNY")
	if message := formatPaymentSuccessAnnouncement(legacy); !strings.Contains(message, "提供商: EZPay") || !strings.Contains(message, "渠道: 支付宝") {
		t.Errorf("legacy EZPay fallback announcement = %q", message)
	}
}

func TestPaymentAnnouncementWorkerSkipRetryAndDelivery(t *testing.T) {
	t.Parallel()

	sendFailure := errors.New("telegram temporarily unavailable")
	tests := []struct {
		name       string
		setting    string
		settingErr error
		sendErr    error
		wantErr    bool
		wantCalls  int
	}{
		{name: "missing channel skips"},
		{name: "settings failure retries", settingErr: errors.New("settings unavailable"), wantErr: true},
		{name: "invalid channel retries", setting: "channel", wantErr: true},
		{name: "Telegram failure retries", setting: "-100123", sendErr: sendFailure, wantErr: true, wantCalls: 1},
		{name: "success", setting: "-100123", wantCalls: 1},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			settings := &paymentAnnouncementSettingsStub{value: test.setting, err: test.settingErr}
			sender := &paymentAnnouncementSenderStub{err: test.sendErr}
			worker := NewPaymentAnnouncementWorker(settings, sender)
			err := worker.HandleOutbox(context.Background(), paymentAnnouncementJob(t, paymentAnnouncementFixture("ezpay", "wxpay", "12.34", "CNY")))
			if (err != nil) != test.wantErr {
				t.Fatalf("HandleOutbox() error = %v, want error %t", err, test.wantErr)
			}
			if sender.calls != test.wantCalls {
				t.Fatalf("SendMarkdownV2Message() calls = %d, want %d", sender.calls, test.wantCalls)
			}
			if test.wantCalls == 1 && sender.chatID != -100123 {
				t.Fatalf("SendMarkdownV2Message() chat ID = %d", sender.chatID)
			}
			if test.sendErr != nil && !errors.Is(err, sendFailure) {
				t.Fatalf("HandleOutbox() error = %v, want wrapped send failure", err)
			}
		})
	}
}

func paymentAnnouncementFixture(provider, channel, amount, currency string) jobpayload.PaymentSuccessAnnouncement {
	return jobpayload.PaymentSuccessAnnouncement{
		OrderID: "order-1", Provider: provider, Channel: channel, TXBMinor: 2_500,
		PayableAmount: amount, PayableCurrency: currency, Username: "@ada",
	}
}

func paymentAnnouncementJob(t *testing.T, payload jobpayload.PaymentSuccessAnnouncement) model.OutboxJob {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(): %v", err)
	}
	return model.OutboxJob{Kind: jobpayload.PaymentSuccessAnnouncementKind, Payload: string(encoded)}
}

type paymentAnnouncementSettingsStub struct {
	value string
	err   error
}

func (s *paymentAnnouncementSettingsStub) Optional(context.Context, string) (string, error) {
	return s.value, s.err
}

type paymentAnnouncementSenderStub struct {
	chatID int64
	calls  int
	err    error
}

func (s *paymentAnnouncementSenderStub) SendMarkdownV2Message(_ context.Context, chatID, _ int64, _ string) error {
	s.calls++
	s.chatID = chatID
	return s.err
}
